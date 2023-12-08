package metamorph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracingLog "github.com/opentracing/opentracing-go/log"
	"github.com/ordishs/go-utils/stat"
	"github.com/ordishs/gocore"
)

const (
	// number of times we will retry announcing transaction if we haven't seen it on the network
	MaxRetries = 15
	// length of interval for checking transactions if they are seen on the network
	// if not we resend them again for a few times
	unseenTransactionRebroadcastingInterval = 60
	processExpiredSeenTxsIntervalDefault    = 5 * time.Minute
	mapExpiryTimeDefault                    = 24 * time.Hour
	LogLevelDefault                         = slog.LevelInfo

	failedToUpdateStatus       = "Failed to update status"
	dataRetentionPeriodDefault = 14 * 24 * time.Hour // 14 days
)

type Processor struct {
	store                store.MetamorphStore
	ProcessorResponseMap *ProcessorResponseMap
	pm                   p2p.PeerManagerI
	btc                  blocktx.ClientI
	logger               *slog.Logger
	logFile              string
	mapExpiryTime        time.Duration
	dataRetentionPeriod  time.Duration
	now                  func() time.Time

	processExpiredSeenTxsInterval time.Duration
	processExpiredSeenTxsTicker   *time.Ticker

	processExpiredTxsTicker *time.Ticker

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

type Option func(f *Processor)

func NewProcessor(s store.MetamorphStore, pm p2p.PeerManagerI, btc blocktx.ClientI, opts ...Option) (*Processor, error) {
	if s == nil {
		return nil, errors.New("store cannot be nil")
	}

	if pm == nil {
		return nil, errors.New("peer manager cannot be nil")
	}

	p := &Processor{
		startTime:               time.Now().UTC(),
		store:                   s,
		pm:                      pm,
		btc:                     btc,
		dataRetentionPeriod:     dataRetentionPeriodDefault,
		mapExpiryTime:           mapExpiryTimeDefault,
		now:                     time.Now,
		processExpiredTxsTicker: time.NewTicker(unseenTransactionRebroadcastingInterval * time.Second),

		processExpiredSeenTxsInterval: processExpiredSeenTxsIntervalDefault,

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

	p.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault})).With(slog.String("service", "mtm"))

	// apply options to processor
	for _, opt := range opts {
		opt(p)
	}

	p.ProcessorResponseMap = NewProcessorResponseMap(p.mapExpiryTime, WithLogFile(p.logFile), WithNowResponseMap(p.now))
	p.processExpiredSeenTxsTicker = time.NewTicker(p.processExpiredSeenTxsInterval)

	p.logger.Info("Starting processor", slog.Duration("cacheExpiryTime", p.mapExpiryTime))

	// Start a goroutine to resend transactions that have not been seen on the network
	go p.processExpiredTransactions()
	go p.processExpiredSeenTransactions()

	gocore.AddAppPayloadFn("mtm", func() interface{} {
		return p.GetStats(false)
	})

	_ = newPrometheusCollector(p)

	return p, nil
}

func (p *Processor) Set(ctx context.Context, req *ProcessorRequest) error {
	// we need to decouple the Context from the request, so that we don't get cancelled
	// when the request is cancelled
	callerSpan := opentracing.SpanFromContext(ctx)
	newctx := opentracing.ContextWithSpan(context.Background(), callerSpan)
	_, spanCtx := opentracing.StartSpanFromContext(newctx, "Processor:processTransaction")
	return p.store.Set(spanCtx, req.Data.Hash[:], req.Data)
}

// Shutdown closes all channels and goroutines gracefully
func (p *Processor) Shutdown() {
	p.logger.Info("Shutting down processor")

	err := p.unlockItems()
	if err != nil {
		p.logger.Error("Failed to unlock all hashes", slog.String("err", err.Error()))
	}
	p.processExpiredSeenTxsTicker.Stop()
	p.processExpiredTxsTicker.Stop()
	p.ProcessorResponseMap.Close()
}

func (p *Processor) unlockItems() error {
	items := p.ProcessorResponseMap.Items()
	hashes := make([]*chainhash.Hash, len(items))
	index := 0
	for key := range items {
		hash, err := chainhash.NewHash(key.CloneBytes())
		if err != nil {
			return err
		}
		hashes[index] = hash
		index++
	}

	p.logger.Info("unlocking items", slog.Int("number", len(hashes)))
	return p.store.SetUnlocked(context.Background(), hashes)
}

func (p *Processor) processExpiredSeenTransactions() {
	// filterFunc returns true if the transaction has not been seen on the network
	filterFunc := func(processorResp *processor_response.ProcessorResponse) bool {
		return processorResp.GetStatus() == metamorph_api.Status_SEEN_ON_NETWORK && p.now().Sub(processorResp.Start) > p.processExpiredSeenTxsInterval
	}

	// Check transactions that have been seen on the network, but haven't been marked as mined
	// The Items() method will return a copy of the map, so we can iterate over it without locking
	for range p.processExpiredSeenTxsTicker.C {
		p.logger.Debug("processing expired seen transactions")

		expiredTransactionItems := p.ProcessorResponseMap.Items(filterFunc)
		if len(expiredTransactionItems) == 0 {
			continue
		}

		p.logger.Debug(fmt.Sprintf("getting transaction blocks from blocktx for %d transactions", len(expiredTransactionItems)))

		transactions := &blocktx_api.Transactions{}
		txs := make([]*blocktx_api.Transaction, len(expiredTransactionItems))
		index := 0
		for _, item := range expiredTransactionItems {
			txs[index] = &blocktx_api.Transaction{Hash: item.Hash.CloneBytes()}
			index++
			transactions.Transactions = txs
		}

		blockTransactions, err := p.btc.GetTransactionBlocks(context.Background(), transactions)
		if err != nil {
			p.logger.Error("failed to get transaction blocks from blocktx", slog.String("err", err.Error()))
			return
		}

		for _, blockTxs := range blockTransactions.TransactionBlocks {
			blockHash, err := chainhash.NewHash(blockTxs.BlockHash)
			if err != nil {
				p.logger.Error("failed to parse block hash", slog.String("err", err.Error()))
				continue
			}
			_, err = p.SendStatusMinedForTransaction((*chainhash.Hash)(blockTxs.TransactionHash), blockHash, blockTxs.BlockHeight)
			if err != nil {
				p.logger.Error("failed to send status mined for tx", slog.String("err", err.Error()))
			}
		}
	}
}

func (p *Processor) processExpiredTransactions() {
	// filterFunc returns true if the transaction has not been seen on the network
	filterFunc := func(procResp *processor_response.ProcessorResponse) bool {
		return (procResp.GetStatus() < metamorph_api.Status_SEEN_ON_NETWORK || procResp.GetStatus() == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL) && p.now().Sub(procResp.Start) > unseenTransactionRebroadcastingInterval*time.Second
	}

	// Resend transactions that have not been seen on the network
	// The Items() method will return a copy of the map, so we can iterate over it without locking
	for range p.processExpiredTxsTicker.C {
		expiredTransactionItems := p.ProcessorResponseMap.Items(filterFunc)
		if len(expiredTransactionItems) > 0 {
			p.logger.Info("Resending expired transactions", slog.Int("number", len(expiredTransactionItems)))
			for txID, item := range expiredTransactionItems {
				startTime := time.Now()
				retries := item.GetRetries()
				item.IncrementRetry()

				if retries > MaxRetries {
					// Sending GETDATA to peers to see if they have it
					p.logger.Debug("Re-getting expired tx", slog.String("hash", txID.String()))
					p.pm.RequestTransaction(item.Hash)
					item.AddLog(
						item.Status,
						"expired",
						"Sent GETDATA for transaction",
					)
				} else {
					p.logger.Debug("Re-announcing expired tx", slog.String("hash", txID.String()))
					p.pm.AnnounceTransaction(item.Hash, item.AnnouncedPeers)
					item.AddLog(
						metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						"expired",
						"Re-announced expired tx",
					)
				}

				p.retries.AddDuration(time.Since(startTime))
			}
		}
	}
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

func (p *Processor) deleteExpired(record *store.StoreData) (recordDeleted bool) {
	if p.now().Sub(record.StoredAt) <= p.dataRetentionPeriod {
		return recordDeleted
	}

	p.logger.Debug("deleting transaction from storage", slog.String("hash", record.Hash.String()), slog.String("status", metamorph_api.Status_name[int32(record.Status)]), slog.Time("storage date", record.StoredAt))

	err := p.store.Del(context.Background(), record.Hash[:])
	if err != nil {
		p.logger.Error("failed to delete transaction", slog.String("hash", record.Hash.String()), slog.String("err", err.Error()))
		return recordDeleted
	}
	recordDeleted = true

	return recordDeleted
}

func (p *Processor) LoadUnmined() {
	span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:LoadUnmined")
	defer span.Finish()

	err := p.store.GetUnmined(spanCtx, func(record *store.StoreData) {

		if !p.store.IsCentralised() && p.deleteExpired(record) {
			return
		}

		// add the records we have in the database, but that have not been processed, to the mempool watcher
		pr := processor_response.NewProcessorResponseWithStatus(record.Hash, record.Status)
		pr.NoStats = true
		pr.Start = record.StoredAt

		p.ProcessorResponseMap.Set(record.Hash, pr)

		switch record.Status {
		case metamorph_api.Status_STORED:
			// announce the transaction to the network
			pr.SetPeers(p.pm.AnnounceTransaction(record.Hash, nil))

			err := p.store.UpdateStatus(spanCtx, record.Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "")
			if err != nil {
				p.logger.Error(failedToUpdateStatus, slog.String("hash", record.Hash.String()), slog.String("err", err.Error()))
			}
		case metamorph_api.Status_ANNOUNCED_TO_NETWORK:
			// we only announced the transaction, but we did not receive a SENT_TO_NETWORK response
			// let's send a GETDATA message to the network to check whether the transaction is actually there
			p.pm.RequestTransaction(record.Hash)
		case metamorph_api.Status_SENT_TO_NETWORK:
			p.pm.RequestTransaction(record.Hash)
		case metamorph_api.Status_SEEN_ON_NETWORK:
			// could it already be mined, and we need to get it from BlockTx?
			transactionResponse, err := p.btc.GetTransactionBlock(context.Background(), &blocktx_api.Transaction{Hash: record.Hash[:]})

			if err != nil {
				p.logger.Error("failed to get transaction block", slog.String("hash", record.Hash.String()), slog.String("err", err.Error()))
				return
			}

			if transactionResponse == nil || transactionResponse.BlockHeight <= 0 {
				return
			}

			// we have a mined transaction, let's update the status
			var blockHash *chainhash.Hash
			blockHash, err = chainhash.NewHash(transactionResponse.BlockHash)
			if err != nil {
				p.logger.Error("Failed to convert block hash", slog.String("err", err.Error()))
				return
			}

			_, err = p.SendStatusMinedForTransaction(record.Hash, blockHash, transactionResponse.BlockHeight)
			if err != nil {
				p.logger.Error("Failed to update status for mined transaction", slog.String("err", err.Error()))
			}
		}
	})
	if err != nil {
		p.logger.Error("Failed to iterate through stored transactions", slog.String("err", err.Error()))
	}
}

func (p *Processor) SendStatusMinedForTransaction(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) (bool, error) {
	span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:SendStatusMinedForTransaction")
	defer span.Finish()

	resp, ok := p.ProcessorResponseMap.Get(hash)
	if !ok {
		return false, fmt.Errorf("failed to get tx %s from response map", hash.String())
	}

	resp.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
		Status: metamorph_api.Status_MINED,
		Source: "blocktx",
		UpdateStore: func() error {
			return p.store.UpdateMined(spanCtx, hash, blockHash, blockHeight)
		},
		Callback: func(err error) {
			if err != nil {
				p.logger.Error(failedToUpdateStatus, slog.String("hash", hash.String()), slog.String("err", err.Error()))
				return
			}

			if !resp.NoStats {
				p.mined.AddDuration(time.Since(resp.Start))
			}

			resp.Close()
			p.ProcessorResponseMap.Delete(hash)

			data, _ := p.store.Get(spanCtx, hash[:])
			go SendCallback(p.logger, p.store, data)
		},
	})

	return true, nil
}

var (
	statusValueMap = map[metamorph_api.Status]int{
		metamorph_api.Status_UNKNOWN:                0,
		metamorph_api.Status_QUEUED:                 1,
		metamorph_api.Status_RECEIVED:               2,
		metamorph_api.Status_STORED:                 3,
		metamorph_api.Status_ANNOUNCED_TO_NETWORK:   4,
		metamorph_api.Status_REQUESTED_BY_NETWORK:   5,
		metamorph_api.Status_SENT_TO_NETWORK:        6,
		metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL: 7,
		metamorph_api.Status_ACCEPTED_BY_NETWORK:    8,
		metamorph_api.Status_SEEN_ON_NETWORK:        9,
		metamorph_api.Status_REJECTED:               10,
		metamorph_api.Status_MINED:                  11,
		metamorph_api.Status_CONFIRMED:              12,
	}
)

func (p *Processor) SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, source string, statusErr error) (bool, error) {
	processorResponse, ok := p.ProcessorResponseMap.Get(hash)
	if !ok {
		return false, nil
	}

	// Do not overwrite a higher value status with a lower or equal value status
	if statusValueMap[status] <= statusValueMap[processorResponse.Status] {
		p.logger.Info("Status not updated for tx", slog.String("status", status.String()), slog.String("previous status", processorResponse.Status.String()), slog.String("hash", hash.String()))

		return false, nil
	}

	span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:SendStatusForTransaction")
	defer span.Finish()

	statusUpdate := &processor_response.ProcessorResponseStatusUpdate{
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
				p.logger.Error(failedToUpdateStatus, slog.String("hash", hash.String()), slog.String("err", err.Error()))
				return
			}

			p.logger.Debug("Status reported for tx", slog.String("status", status.String()), slog.String("hash", hash.String()))

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
				p.ProcessorResponseMap.Delete(hash)

			case metamorph_api.Status_REJECTED:
				p.logger.Warn("transaction rejected", slog.String("status", status.String()), slog.String("hash", hash.String()))

				p.rejected.AddDuration(source, time.Since(processorResponse.Start))
			}
		},
	}

	processorResponse.UpdateStatus(statusUpdate)

	return true, nil
}

func (p *Processor) ProcessTransaction(ctx context.Context, req *ProcessorRequest) {
	startNanos := time.Now().UnixNano()

	// we need to decouple the Context from the request, so that we don't get cancelled
	// when the request is cancelled
	callerSpan := opentracing.SpanFromContext(ctx)
	newctx := opentracing.ContextWithSpan(context.Background(), callerSpan)
	span, spanCtx := opentracing.StartSpanFromContext(newctx, "Processor:processTransaction")
	defer span.Finish()

	p.queuedCount.Add(1)

	p.logger.Debug("Adding channel", slog.String("hash", req.Data.Hash.String()))

	processorResponse := processor_response.NewProcessorResponseWithChannel(req.Data.Hash, req.ResponseChannel)

	// STEP 1: RECEIVED
	processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
		Status: metamorph_api.Status_RECEIVED,
		Source: "processor",
		Callback: func(err error) {
			if err != nil {
				p.logger.Error("Error received when setting status", slog.String("status", metamorph_api.Status_RECEIVED.String()), slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
				return
			}

			// STEP 2: STORED
			processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
				Status: metamorph_api.Status_STORED,
				Source: "processor",
				UpdateStore: func() error {
					return p.store.Set(spanCtx, req.Data.Hash[:], req.Data)
				},
				Callback: func(err error) {
					if err != nil {
						span.SetTag(string(ext.Error), true)
						p.logger.Error("Failed to store transaction", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
						span.LogFields(tracingLog.Error(err))
						return
					}

					// Add this transaction to the map of transactions that we are processing
					p.ProcessorResponseMap.Set(req.Data.Hash, processorResponse)

					p.stored.AddDuration(time.Since(processorResponse.Start))

					// STEP 3: ANNOUNCED_TO_NETWORK
					peers := p.pm.AnnounceTransaction(req.Data.Hash, nil)
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
							return p.store.UpdateStatus(spanCtx, req.Data.Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "")
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
