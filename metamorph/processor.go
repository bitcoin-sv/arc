package metamorph

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/processor_response"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/stat"

	"github.com/ordishs/gocore"
)

type ProcessorStats struct {
	StartTime          time.Time
	UptimeMillis       string
	QueueLength        int32
	QueuedCount        int32
	Stored             *stat.AtomicStat
	AnnouncedToNetwork *stat.AtomicStats
	RequestedByNetwork *stat.AtomicStats
	SentToNetwork      *stat.AtomicStats
	AcceptedByNetwork  *stat.AtomicStats
	SeenOnNetwork      *stat.AtomicStats
	Rejected           *stat.AtomicStats
	Mined              *stat.AtomicStat
	Retries            *stat.AtomicStat
	ChannelMapSize     int32
}

type Processor struct {
	ch                   chan *ProcessorRequest
	store                store.MetamorphStore
	cbChannel            chan *callbacker_api.Callback
	processorResponseMap *ProcessorResponseMap
	pm                   p2p.PeerManagerI
	logger               utils.Logger
	metamorphAddress     string
	errorLogFile         string
	errorLogWorker       chan *processor_response.ProcessorResponse

	startTime          time.Time
	queueLength        atomic.Int32
	queuedCount        atomic.Int32
	stored             *stat.AtomicStat
	announcedToNetwork *stat.AtomicStats
	requestedByNetwork *stat.AtomicStats
	sentToNetwork      *stat.AtomicStats
	acceptedByNetwork  *stat.AtomicStats
	seenOnNetwork      *stat.AtomicStats
	rejected           *stat.AtomicStats
	mined              *stat.AtomicStat
	retries            *stat.AtomicStat
}

func NewProcessor(s store.MetamorphStore, pm p2p.PeerManagerI, metamorphAddress string,
	cbChannel chan *callbacker_api.Callback) *Processor {
	if s == nil {
		panic("store cannot be nil")
	}
	if pm == nil {
		panic("peer manager cannot be nil")
	}

	logLevel, _ := gocore.Config().Get("logLevel")
	logger := gocore.Log("proc", gocore.NewLogLevelFromString(logLevel))

	mapExpiryStr, _ := gocore.Config().Get("processorCacheExpiryTime", "24h")
	mapExpiry, err := time.ParseDuration(mapExpiryStr)
	if err != nil {
		logger.Fatalf("Invalid processorCacheExpiryTime: %s", mapExpiryStr)
	}

	logger.Infof("Starting processor with cache expiry of %s", mapExpiryStr)

	p := &Processor{
		startTime:            time.Now().UTC(),
		ch:                   make(chan *ProcessorRequest),
		store:                s,
		cbChannel:            cbChannel,
		processorResponseMap: NewProcessorResponseMap(mapExpiry),
		pm:                   pm,
		logger:               logger,
		metamorphAddress:     metamorphAddress,

		stored:             stat.NewAtomicStat(),
		announcedToNetwork: stat.NewAtomicStats(),
		requestedByNetwork: stat.NewAtomicStats(),
		sentToNetwork:      stat.NewAtomicStats(),
		acceptedByNetwork:  stat.NewAtomicStats(),
		seenOnNetwork:      stat.NewAtomicStats(),
		rejected:           stat.NewAtomicStats(),
		mined:              stat.NewAtomicStat(),
		retries:            stat.NewAtomicStat(),
	}

	// Start a goroutine to resend transactions that have not been seen on the network
	go p.processExpiredTransactions()

	p.errorLogFile, _ = gocore.Config().Get("metamorph_logErrorFile") //, "./data/metamorph.log")
	if p.errorLogFile != "" {
		p.errorLogWorker = make(chan *processor_response.ProcessorResponse)
		go p.errorLogWriter()
	}

	gocore.AddAppPayloadFn("mtm", func() interface{} {
		return p.GetStats(false)
	})

	_ = newPrometheusCollector(p)

	return p
}

func (p *Processor) GetMetamorphAddress() string {
	return p.metamorphAddress
}

func (p *Processor) errorLogWriter() {
	dir := path.Dir(p.errorLogFile)
	_ = os.MkdirAll(dir, 0777)

	f, err := os.OpenFile(p.errorLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		p.logger.Errorf("error opening log file: %s", err.Error())
	}
	defer f.Close()

	var storeData *store.StoreData
	for pr := range p.errorLogWorker {
		storeData, err = p.store.Get(context.Background(), pr.Hash[:])
		if err != nil {
			p.logger.Errorf("error getting tx from store: %s", err.Error())
			continue
		}
		_, err = f.WriteString(fmt.Sprintf("%v,%s,%s\n",
			pr.Hash,
			pr.Err.Error(),
			hex.EncodeToString(storeData.RawTx),
		))
		if err != nil {
			p.logger.Errorf("error writing to log file: %s", err.Error())
		}
	}
}

func (p *Processor) processExpiredTransactions() {
	// filterFunc returns true if the transaction has not been seen on the network
	filterFunc := func(p *processor_response.ProcessorResponse) bool {
		return p.GetStatus() < metamorph_api.Status_SEEN_ON_NETWORK && time.Since(p.Start) < 60*time.Second
	}

	// Resend transactions that have not been seen on the network every 60 seconds
	// The Items() method will return a copy of the map, so we can iterate over it without locking
	for range time.NewTicker(60 * time.Second).C {
		expiredTransactionItems := p.processorResponseMap.Items(filterFunc)
		if len(expiredTransactionItems) > 0 {
			p.logger.Infof("Resending %d expired transactions", len(expiredTransactionItems))
			for txID, item := range expiredTransactionItems {
				startTime := time.Now()
				retries := item.GetRetries()
				p.logger.Debugf("Resending expired tx: %s (%d retries)", txID, retries)
				if retries >= 3 {
					continue
				} else if retries >= 2 {
					// check for it in blocktx, it might have been mined
					p.logger.Debugf("Transaction %s has been retried 4 times, not resending", txID)
					continue
				} else if retries >= 1 {
					// retried announcing 2 times, now sending GETDATA to peers to see if they have it
					p.logger.Debugf("Re-getting expired tx: %s", txID)
					p.pm.RequestTransaction(item.Hash)
					item.AddLog(
						item.Status,
						"expired",
						"Sent GETDATA for transaction",
					)
				} else {
					p.logger.Debugf("Re-announcing expired tx: %s", txID)
					p.pm.AnnounceTransaction(item.Hash, item.AnnouncedPeers)
					item.AddLog(
						metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						"expired",
						"Re-announced expired tx",
					)
				}

				p.retries.AddDuration(time.Since(startTime))

				item.IncrementRetry()
			}
		}
	}
}

func (p *Processor) SetLogger(logger utils.Logger) {
	p.logger = logger
}

// GetPeers returns a list of connected and a list of disconnected peers
func (p *Processor) GetPeers() ([]string, []string) {
	peers := p.pm.GetPeers()
	peersConnected := make([]string, 0, len(peers))
	peersDisconnected := make([]string, 0, len(peers))
	for _, peer := range peers {
		if peer.Connected() {
			peersConnected = append(peersConnected, peer.String())
		} else {
			peersDisconnected = append(peersDisconnected, peer.String())
		}
	}

	return peersConnected, peersDisconnected
}

func (p *Processor) LoadUnmined() {
	span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:LoadUnmined")
	defer span.Finish()

	err := p.store.GetUnmined(spanCtx, func(record *store.StoreData) {
		// add the records we have in the database, but that have not been processed, to the mempool watcher
		pr := processor_response.NewProcessorResponseWithStatus(record.Hash, record.Status)
		pr.NoStats = true

		p.processorResponseMap.Set(record.Hash, pr)

		if record.Status == metamorph_api.Status_STORED {
			// announce the transaction to the network
			pr.SetPeers(p.pm.AnnounceTransaction(record.Hash, nil))

			err := p.store.UpdateStatus(spanCtx, record.Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "")
			if err != nil {
				p.logger.Errorf("Error updating status for %v: %v", record.Hash, err)
			}
		} else if record.Status == metamorph_api.Status_ANNOUNCED_TO_NETWORK {
			// we only announced the transaction, but we did not receive a SENT_TO_NETWORK response

			// could it already be mined, and we need to get it from BlockTx?

			// let's send a GETDATA message to the network to check whether the transaction is actually there
			p.pm.RequestTransaction(record.Hash)
		} else if record.Status <= metamorph_api.Status_SENT_TO_NETWORK {
			p.pm.RequestTransaction(record.Hash)
		}
	})
	if err != nil {
		p.logger.Errorf("Error iterating through stored transactions: %v", err)
	}
}

func (p *Processor) SendStatusMinedForTransaction(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) (bool, error) {
	span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:SendStatusMinedForTransaction")
	defer span.Finish()

	resp, ok := p.processorResponseMap.Get(hash)
	if ok {
		resp.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
			Status: metamorph_api.Status_MINED,
			Source: "blocktx",
			UpdateStore: func() error {
				return p.store.UpdateMined(spanCtx, hash, blockHash, blockHeight)
			},
			Callback: func(err error) {
				if err != nil {
					p.logger.Errorf("Error updating status for %v: %v", hash, err)
					return
				}

				if !resp.NoStats {
					p.mined.AddDuration(time.Since(resp.Start))
				}

				resp.Close()
				p.processorResponseMap.Delete(hash)

				if p.cbChannel != nil {
					data, _ := p.store.Get(spanCtx, hash[:])

					if data != nil && data.CallbackUrl != "" {
						p.cbChannel <- &callbacker_api.Callback{
							Hash:        data.Hash[:],
							Url:         data.CallbackUrl,
							Token:       data.CallbackToken,
							Status:      int32(data.Status),
							BlockHash:   data.BlockHash[:],
							BlockHeight: data.BlockHeight,
						}
					}
				}

			},
		})
		return true, nil
	}

	return false, nil
}

func (p *Processor) SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, source string, statusErr error) (bool, error) {
	processorResponse, ok := p.processorResponseMap.Get(hash)
	if ok {
		span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:SendStatusForTransaction")
		defer span.Finish()

		processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
			Status:    status,
			Source:    source,
			StatusErr: statusErr,
			UpdateStore: func() error {
				// we have cached this transaction, so process accordingly
				rejectReason := ""
				if statusErr != nil {
					rejectReason = statusErr.Error()
				}

				return p.store.UpdateStatus(spanCtx, hash, status, rejectReason)
			},
			IgnoreCallback: processorResponse.NoStats, // do not do this callback if we are not keeping stats
			Callback: func(err error) {
				if err != nil {
					p.logger.Errorf("Error updating status for %v: %v", hash, err)
					return
				} else {
					p.logger.Debugf("Status %s reported for tx %v", status, hash)
				}

				switch status {

				case metamorph_api.Status_REQUESTED_BY_NETWORK:
					p.requestedByNetwork.AddDuration(source, time.Since(processorResponse.Start))

				case metamorph_api.Status_SENT_TO_NETWORK:
					p.sentToNetwork.AddDuration(source, time.Since(processorResponse.Start))

				case metamorph_api.Status_ACCEPTED_BY_NETWORK:
					p.acceptedByNetwork.AddDuration(source, time.Since(processorResponse.Start))

				case metamorph_api.Status_SEEN_ON_NETWORK:
					p.seenOnNetwork.AddDuration(source, time.Since(processorResponse.Start))

				case metamorph_api.Status_MINED:
					p.mined.AddDuration(time.Since(processorResponse.Start))
					processorResponse.Close()
					p.processorResponseMap.Delete(hash)

				case metamorph_api.Status_REJECTED:
					// log transaction to the error log
					if p.errorLogFile != "" && p.errorLogWorker != nil {
						utils.SafeSend(p.errorLogWorker, processorResponse)
					}

					p.rejected.AddDuration(source, time.Since(processorResponse.Start))
					// processorResponse.Close()
					// p.processorResponseMap.Delete(hashStr)
				}
			},
		})

	} else if status > metamorph_api.Status_MINED {
		span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:SendStatusForTransaction:Mined")
		defer span.Finish()

		// we only care about statuses that are larger than mined (ie Rejected), since mined is the last status
		// and the transactions should be in the map until they are mined
		if statusErr != nil {
			// Print the error along with the status message
			p.logger.Debugf("Received status %s for tx %v: %s", status.String(), hash, statusErr.Error())
		} else {
			p.logger.Debugf("Received status %s for tx %v", status.String(), hash)
		}

		// This is coming from zmq, after the transaction has been deleted from our processorResponseMap
		// It could be a "seen", "confirmed", "mined" or "rejected" status, but should finalize the tx

		rejectReason := ""
		if statusErr != nil {
			rejectReason = statusErr.Error()
		}

		if err := p.store.UpdateStatus(spanCtx, hash, status, rejectReason); err != nil {
			if err == store.ErrNotFound {
				return false, nil
			}
			p.logger.Errorf("Error updating status for %v: %v", hash, err)
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (p *Processor) ProcessTransaction(req *ProcessorRequest) {
	startNanos := time.Now().UnixNano()

	// we need to decouple the context from the request, so that we don't get cancelled
	// when the request is cancelled
	var span opentracing.Span
	var spanCtx context.Context
	if opentracing.IsGlobalTracerRegistered() {
		callerSpan := opentracing.SpanFromContext(req.ctx)
		ctx := opentracing.ContextWithSpan(context.Background(), callerSpan)
		span, spanCtx = opentracing.StartSpanFromContext(ctx, "Processor:processTransaction")
		defer span.Finish()
	} else {
		spanCtx = req.ctx
	}

	p.queuedCount.Add(1)

	p.logger.Debugf("Adding channel for %v", req.Hash)

	processorResponse := processor_response.NewProcessorResponseWithChannel(req.Hash, req.ResponseChannel)

	// STEP 1: RECEIVED
	processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
		Status: metamorph_api.Status_RECEIVED,
		Source: "processor",
		Callback: func(err error) {
			if err != nil {
				p.logger.Errorf("Error received when setting status of %v to %s", req.Hash, metamorph_api.Status_RECEIVED.String())
				return
			}

			// STEP 2: STORED
			processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
				Status: metamorph_api.Status_STORED,
				Source: "processor",
				UpdateStore: func() error {
					return p.store.Set(spanCtx, req.Hash[:], req.StoreData)
				},
				Callback: func(err error) {
					if err != nil {
						p.logger.Errorf("Error storing transaction %v: %v", req.Hash, err)
						if span != nil {
							span.SetTag(string(ext.Error), true)
							span.LogFields(log.Error(err))
						}
						return
					}

					// Add this transaction to the map of transactions that we are processing
					p.processorResponseMap.Set(req.Hash, processorResponse)

					p.stored.AddDuration(time.Since(processorResponse.Start))

					// STEP 3: ANNOUNCED_TO_NETWORK
					peers := p.pm.AnnounceTransaction(req.Hash, nil)
					processorResponse.SetPeers(peers)

					peersStr := make([]string, 0, len(peers))
					if len(peers) > 0 {
						for _, peer := range peers {
							peersStr = append(peersStr, peer.String())
						}
					}

					processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
						Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						Source: strings.Join(peersStr, ", "),
						UpdateStore: func() error {
							return p.store.UpdateStatus(spanCtx, req.Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "")
						},
						Callback: func(err error) {
							duration := time.Since(processorResponse.Start)
							for _, peerStr := range peersStr {
								p.announcedToNetwork.AddDuration(peerStr, duration)
							}

							gocore.NewStat("processor").AddTime(startNanos)
						},
					})

				},
			})
		},
	})
}
