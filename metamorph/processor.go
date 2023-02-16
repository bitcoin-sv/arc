package metamorph

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/stat"

	"github.com/ordishs/gocore"
)

func init() {
	// Create a stat for processor that ignores children before any other stat is created
	gocore.NewStat("processor", true)
	gocore.NewStat("processor - async")
}

type StatusAndError struct {
	Hash   []byte
	Status metamorph_api.Status
	Err    error
}

type ProcessorStats struct {
	StartTime          time.Time
	UptimeMillis       string
	WorkerCount        int
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
	ChannelMapSize     int32
}

type Processor struct {
	ch                   chan *ProcessorRequest
	store                store.MetamorphStore
	registerCh           chan *blocktx_api.TransactionAndSource
	cbChannel            chan *callbacker_api.Callback
	processorResponseMap *ProcessorResponseMap
	pm                   p2p.PeerManagerI
	logger               utils.Logger
	metamorphAddress     string
	errorLogFile         string
	errorLogWorker       chan *ProcessorResponse

	startTime          time.Time
	workerCount        int
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
}

func NewProcessor(workerCount int, s store.MetamorphStore, pm p2p.PeerManagerI, metamorphAddress string,
	registerCh chan *blocktx_api.TransactionAndSource, cbChannel chan *callbacker_api.Callback) *Processor {
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

	logger.Infof("Starting processor with %d workers and cache expiry of %s", workerCount, mapExpiryStr)

	p := &Processor{
		startTime:            time.Now().UTC(),
		ch:                   make(chan *ProcessorRequest),
		store:                s,
		registerCh:           registerCh,
		cbChannel:            cbChannel,
		processorResponseMap: NewProcessorResponseMap(mapExpiry),
		workerCount:          workerCount,
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
	}

	// Start a goroutine to resend transactions that have not been seen on the network
	go p.processExpiredTransactions()

	p.errorLogFile, _ = gocore.Config().Get("metamorph_logErrorFile") //, "./data/metamorph.log")
	if p.errorLogFile != "" {
		p.errorLogWorker = make(chan *ProcessorResponse)
		go p.errorLogWriter()
	}

	for i := 0; i < workerCount; i++ {
		go p.process(i)
	}

	gocore.AddAppPayloadFn("mtm", func() interface{} {
		return p.GetStats()
	})

	_ = newPrometheusCollector(p)

	return p
}

func (p *Processor) errorLogWriter() {
	f, err := os.OpenFile(p.errorLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		p.logger.Errorf("error opening log file: %s", err.Error())
	}
	defer f.Close()

	var storeData *store.StoreData
	for pr := range p.errorLogWorker {
		storeData, err = p.store.Get(context.Background(), pr.Hash)
		if err != nil {
			p.logger.Errorf("error getting tx from store: %s", err.Error())
			continue
		}
		_, err = f.WriteString(fmt.Sprintf("%s,%s,%s\n",
			utils.HexEncodeAndReverseBytes(pr.Hash),
			pr.Err.Error(),
			hex.EncodeToString(storeData.RawTx),
		))
		if err != nil {
			log.Printf("error writing to log file: %s", err.Error())
		}
	}
}

func (p *Processor) processExpiredTransactions() {
	// filterFunc returns true if the transaction has not been seen on the network
	filterFunc := func(p *ProcessorResponse) bool {
		return p.GetStatus() < metamorph_api.Status_SEEN_ON_NETWORK
	}

	// Resend transactions that have not been seen on the network every 60 seconds
	// The Items() method will return a copy of the map, so we can iterate over it without locking
	for range time.NewTicker(60 * time.Second).C {
		expiredTransactionItems := p.processorResponseMap.Items(filterFunc)
		if len(expiredTransactionItems) > 0 {
			p.logger.Infof("Resending %d expired transactions", len(expiredTransactionItems))
			for txID, item := range expiredTransactionItems {
				retries := item.GetRetries()
				p.logger.Debugf("Resending expired tx: %s (%d retries)", txID, retries)
				if retries >= 3 {
					continue
				} else if retries >= 2 {
					// TODO what should we do here?
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

func (p *Processor) LoadUnseen() {
	err := p.store.GetUnseen(context.Background(), func(record *store.StoreData) {
		// add the records we have in the database, but that have not been processed, to the mempool watcher
		txIDStr := hex.EncodeToString(bt.ReverseBytes(record.Hash))
		pr := NewProcessorResponseWithStatus(record.Hash, record.Status)
		pr.noStats = true

		p.processorResponseMap.Set(txIDStr, pr)

		if record.Status == metamorph_api.Status_STORED {
			// we only stored the transaction, but maybe did not register it with block tx
			if p.registerCh != nil {
				p.logger.Infof("Sending tx %s to register", txIDStr)
				utils.SafeSend(p.registerCh, &blocktx_api.TransactionAndSource{
					Hash:   record.Hash,
					Source: p.metamorphAddress,
				})
			}

			// announce the transaction to the network
			pr.SetPeers(p.pm.AnnounceTransaction(record.Hash, nil))

			err := p.store.UpdateStatus(context.Background(), record.Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "")
			if err != nil {
				p.logger.Errorf("Error updating status for %x: %v", bt.ReverseBytes(record.Hash), err)
			}
		} else if record.Status >= metamorph_api.Status_ANNOUNCED_TO_NETWORK {
			// we only announced the transaction, but we did not receive a SENT_TO_NETWORK response

			// TODO could it already be mined, and we need to get it from BlockTx?

			// let's send a GETDATA message to the network to check whether the transaction is actually there
			// TODO - get a more efficient way to do this from the node
			// we only need the tx ids, not the whole transaction
			p.pm.RequestTransaction(record.Hash)
		}
	})
	if err != nil {
		p.logger.Errorf("Error iterating through stored transactions: %v", err)
	}
}

func (p *Processor) ProcessTransaction(req *ProcessorRequest) {
	p.queuedCount.Add(1)
	p.queueLength.Add(1)

	p.ch <- req
}

func (p *Processor) SendStatusMinedForTransaction(hash []byte, blockHash []byte, blockHeight int32) (bool, error) {
	hashStr := utils.HexEncodeAndReverseBytes(hash)

	err := p.store.UpdateMined(context.Background(), hash, blockHash, blockHeight)
	if err != nil {
		if err != store.ErrNotFound {
			p.logger.Errorf("Error updating status for %s: %v", hashStr, err)
			return false, err
		}
	} else {
		p.logger.Infof("Status mined reported for tx %s", hashStr)
	}

	// remove the transaction from the tx map, regardless of status
	resp, ok := p.processorResponseMap.Get(hashStr)
	if ok {
		resp.setStatus(metamorph_api.Status_MINED, "blocktx")
		if !resp.noStats {
			p.mined.AddDuration(time.Since(resp.Start))
		}

		p.processorResponseMap.Delete(hashStr)
	}

	if p.cbChannel != nil {
		data, _ := p.store.Get(context.Background(), hash)
		if data != nil && data.CallbackUrl != "" {
			p.cbChannel <- &callbacker_api.Callback{
				Hash:        data.Hash,
				Url:         data.CallbackUrl,
				Token:       data.CallbackToken,
				Status:      int32(data.Status),
				BlockHash:   data.BlockHash,
				BlockHeight: uint64(data.BlockHeight),
			}
		}
	}

	return true, nil
}

func (p *Processor) SendStatusForTransaction(hashStr string, status metamorph_api.Status, source string, statusErr error) (bool, error) {
	processorResponse, ok := p.processorResponseMap.Get(hashStr)
	if ok {
		processorResponse.mu.Lock()
		defer processorResponse.mu.Unlock()

		if processorResponse.Status >= status && statusErr == nil {
			processorResponse.AddLog(status, source, "duplicate")

			return false, nil
		}

		// we have cached this transaction, so process accordingly
		rejectReason := ""
		if statusErr != nil {
			rejectReason = statusErr.Error()
		}

		err := p.store.UpdateStatus(context.Background(), processorResponse.Hash, status, rejectReason)
		if err != nil {
			return false, fmt.Errorf("error storing status for %s: %v", hashStr, err)
		} else {
			p.logger.Infof("Status change reported: %s: %s", utils.HexEncodeAndReverseBytes(processorResponse.Hash), status)
		}

		if statusErr != nil {
			ok = processorResponse.setStatusAndErrorInternal(status, statusErr, source)
		} else {
			ok = processorResponse.setStatusInternal(status, source)
		}

		if !processorResponse.noStats {
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
				p.processorResponseMap.Delete(hashStr)

			case metamorph_api.Status_REJECTED:
				// log transaction to the error log
				if p.errorLogFile != "" && p.errorLogWorker != nil {
					utils.SafeSend(p.errorLogWorker, processorResponse)
				}

				p.rejected.AddDuration(source, time.Since(processorResponse.Start))
				p.processorResponseMap.Delete(hashStr)

			}
		}

		statKey := fmt.Sprintf("%d: %s", status, status.String())
		processorResponse.LastStatusUpdateNanos.Store(gocore.NewStat("processor - async").NewStat(statKey).AddTime(processorResponse.LastStatusUpdateNanos.Load()))

		return ok, nil
	} else if status > metamorph_api.Status_SENT_TO_NETWORK {
		if statusErr != nil {
			// Print the error along with the status message
			p.logger.Debugf("Received status %s for tx %s: %s", status.String(), hashStr, statusErr.Error())
		} else {
			p.logger.Debugf("Received status %s for tx %s", status.String(), hashStr)
		}

		// This is coming from zmq, after the transaction has been deleted from our processorResponseMap
		// It could be a "seen", "confirmed", "mined" or "rejected" status, but should finalize the tx
		hash, err := utils.DecodeAndReverseHexString(hashStr)
		if err != nil {
			p.logger.Errorf("Error decoding txID %s: %v", hashStr, err)
			return false, err
		}

		rejectReason := ""
		if statusErr != nil {
			rejectReason = statusErr.Error()
		}
		err = p.store.UpdateStatus(context.Background(), hash, status, rejectReason)
		if err != nil {
			if err == store.ErrNotFound {
				return false, nil
			}
			p.logger.Errorf("Error updating status for %s: %v", hashStr, err)
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (p *Processor) process(_ int) {
	for req := range p.ch {
		p.processTransaction(req)
	}
}

func (p *Processor) processTransaction(req *ProcessorRequest) {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("processor").AddTime(startNanos)
	}()

	span, _ := opentracing.StartSpanFromContext(req.ctx, "Processor:processTransaction")
	defer span.Finish()

	p.queueLength.Add(-1)

	p.logger.Debugf("Adding channel for %x", bt.ReverseBytes(req.Hash))

	processorResponse := NewProcessorResponseWithChannel(req.Hash, req.ResponseChannel)
	processorResponse.setStatus(metamorph_api.Status_RECEIVED, "processor")

	nextNanos := gocore.NewStat("processor").NewStat("2: RECEIVED").AddTime(startNanos)

	txIDStr := hex.EncodeToString(bt.ReverseBytes(req.Hash))

	if err := p.store.Set(context.Background(), req.Hash, req.StoreData); err != nil {
		p.logger.Errorf("Error storing transaction %s: %v", txIDStr, err)
		processorResponse.setErr(err, "processor")
		return
	}

	p.processorResponseMap.Set(txIDStr, processorResponse)

	p.logger.Debugf("Stored tx %s", txIDStr)

	processorResponse.setStatus(metamorph_api.Status_STORED, "processor")

	nextNanos = gocore.NewStat("processor").NewStat("3: STORED").AddTime(nextNanos)

	p.stored.AddDuration(time.Since(processorResponse.Start))

	if p.registerCh != nil {
		p.logger.Debugf("Sending tx %s to register", txIDStr)
		utils.SafeSend(p.registerCh, &blocktx_api.TransactionAndSource{
			Hash:   req.Hash,
			Source: p.metamorphAddress,
		})
	}

	peers := p.pm.AnnounceTransaction(req.Hash, nil)
	processorResponse.SetPeers(peers)

	peersStr := make([]string, 0, len(peers))
	if len(peers) > 0 {
		for _, peer := range peers {
			peersStr = append(peersStr, peer.String())
		}
	}

	processorResponse.setStatus(metamorph_api.Status_ANNOUNCED_TO_NETWORK, strings.Join(peersStr, ", "))

	next := gocore.NewStat("processor").NewStat("4: ANNOUNCED").AddTime(nextNanos)
	processorResponse.LastStatusUpdateNanos.Store(next)

	duration := time.Since(processorResponse.Start)
	for _, peerStr := range peersStr {
		p.announcedToNetwork.AddDuration(peerStr, duration)
	}

	// update to the latest status of the transaction
	// we have to store in the background, since we do not want to stop the saving, even if the request ctx has stopped
	err := p.store.UpdateStatus(context.Background(), req.Hash, processorResponse.GetStatus(), "")
	if err != nil {
		if err != store.ErrNotFound {
			p.logger.Errorf("Error updating status for %x: %v", bt.ReverseBytes(req.Hash), err)
		}
	}

	gocore.NewStat("processor").NewStat("4: ANNOUNCED stored").AddTime(next)
	// Don't set the lastStatusUpdateNanos here, because we don't want to count the time it takes to store the status
}
