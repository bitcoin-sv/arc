package metamorph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	"github.com/ordishs/go-utils/stat"
	"github.com/ordishs/gocore"
)

const (
	// MaxRetries number of times we will retry announcing transaction if we haven't seen it on the network
	MaxRetries = 15
	// length of interval for checking transactions if they are seen on the network
	// if not we resend them again for a few times
	unseenTransactionRebroadcastingInterval = 60

	processCheckIfMinedIntervalDefault = 1 * time.Minute

	mapExpiryTimeDefault = 24 * time.Hour
	LogLevelDefault      = slog.LevelInfo

	failedToUpdateStatus       = "Failed to update status"
	dataRetentionPeriodDefault = 14 * 24 * time.Hour // 14 days
)

type Processor struct {
	store               store.MetamorphStore
	pm                  p2p.PeerManagerI
	btc                 blocktx.ClientI
	logger              *slog.Logger
	logFile             string
	mapExpiryTime       time.Duration
	dataRetentionPeriod time.Duration
	now                 func() time.Time

	processCheckIfMinedInterval time.Duration
	processCheckIfMinedTicker   *time.Ticker

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

		processCheckIfMinedInterval: processCheckIfMinedIntervalDefault,

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

	p.processCheckIfMinedTicker = time.NewTicker(p.processCheckIfMinedInterval)

	p.logger.Info("Starting processor", slog.Duration("cacheExpiryTime", p.mapExpiryTime))

	// Start a goroutine to resend transactions that have not been seen on the network
	go p.processCheckIfMined()

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

	//err := p.unlockItems()
	//if err != nil {
	//	p.logger.Error("Failed to unlock all hashes", slog.String("err", err.Error()))
	//}
	p.processCheckIfMinedTicker.Stop()
	p.processExpiredTxsTicker.Stop()
}

//func (p *Processor) unlockItems() error {
//	items := p.ProcessorResponseMap.Items()
//	hashes := make([]*chainhash.Hash, len(items))
//	index := 0
//	for key := range items {
//		hash, err := chainhash.NewHash(key.CloneBytes())
//		if err != nil {
//			return err
//		}
//		hashes[index] = hash
//		index++
//	}
//
//	p.logger.Info("unlocking items", slog.Int("number", len(hashes)))
//	return p.store.SetUnlocked(context.Background(), hashes)
//}

func (p *Processor) processCheckIfMined() {
	// filter for transactions which have been at least announced but not mined and which haven't started to be processed longer than a specified amount of time ago
	//filterFunc := func(processorResp *processor_response.ProcessorResponse) bool {
	//	return processorResp.GetStatus() != metamorph_api.Status_MINED &&
	//		processorResp.GetStatus() != metamorph_api.Status_CONFIRMED
	//}

	// Check transactions that have been seen on the network, but haven't been marked as mined
	// The Items() method will return a copy of the map, so we can iterate over it without locking
	for range p.processCheckIfMinedTicker.C {
		mined, err := p.store.GetByStatus(context.TODO(), metamorph_api.Status_MINED)
		if err != nil {
			p.logger.Error("failed to query mined transactions", slog.String("err", err.Error()))
			return
		}

		confirmed, err := p.store.GetByStatus(context.TODO(), metamorph_api.Status_CONFIRMED)
		if err != nil {
			p.logger.Error("failed to query mined transactions", slog.String("err", err.Error()))
			return
		}

		//expiredTransactionItems := p.ProcessorResponseMap.Items(filterFunc)
		//if len(expiredTransactionItems) == 0 {
		//	continue
		//}

		transactions := &blocktx_api.Transactions{}
		for _, item := range append(mined, confirmed...) {
			transactions.Transactions = append(transactions.Transactions, &blocktx_api.Transaction{Hash: item.Hash.CloneBytes()})
		}

		blockTransactions, err := p.btc.GetTransactionBlocks(context.Background(), transactions)
		if err != nil {
			p.logger.Error("failed to get transaction blocks from blocktx", slog.String("err", err.Error()))
			return
		}

		p.logger.Debug("found blocks for transactions", slog.Int("number", len(blockTransactions.GetTransactionBlocks())))

		for _, blockTxs := range blockTransactions.GetTransactionBlocks() {
			blockHash, err := chainhash.NewHash(blockTxs.GetBlockHash())
			if err != nil {
				p.logger.Error("failed to parse block hash", slog.String("err", err.Error()))
				continue
			}

			txHash, err := chainhash.NewHash(blockTxs.GetTransactionHash())
			if err != nil {
				p.logger.Error("failed to parse tx hash", slog.String("err", err.Error()))
				continue
			}
			p.logger.Debug("found block for transaction", slog.String("txhash", txHash.String()), slog.String("blockhash", blockHash.String()))

			err = p.store.UpdateMined(context.TODO(), blockHash, blockHash, blockTxs.GetBlockHeight())
			if err != nil {
				p.logger.Error("failed to send status mined for tx", slog.String("err", err.Error()))
			}
		}
	}
}

//func (p *Processor) processExpiredTransactions() {
//	// filterFunc returns true if the transaction has not been seen on the network
//	filterFunc := func(procResp *processor_response.ProcessorResponse) bool {
//		return (procResp.GetStatus() < metamorph_api.Status_SEEN_ON_NETWORK || procResp.GetStatus() == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL) && p.now().Sub(procResp.Start) > unseenTransactionRebroadcastingInterval*time.Second
//	}
//
//	// Resend transactions that have not been seen on the network
//	// The Items() method will return a copy of the map, so we can iterate over it without locking
//	for range p.processExpiredTxsTicker.C {
//		expiredTransactionItems := p.ProcessorResponseMap.Items(filterFunc)
//
//
//		if len(expiredTransactionItems) > 0 {
//			p.logger.Info("Resending expired transactions", slog.Int("number", len(expiredTransactionItems)))
//			for txID, item := range expiredTransactionItems {
//				startTime := time.Now()
//				retries := item.GetRetries()
//				item.IncrementRetry()
//
//				if retries > MaxRetries {
//					// Sending GETDATA to peers to see if they have it
//					p.logger.Debug("Re-getting expired tx", slog.String("hash", txID.String()))
//					p.pm.RequestTransaction(item.Hash)
//					item.AddLog(
//						item.Status,
//						"expired",
//						"Sent GETDATA for transaction",
//					)
//				} else {
//					p.logger.Debug("Re-announcing expired tx", slog.String("hash", txID.String()))
//					p.pm.AnnounceTransaction(item.Hash, item.AnnouncedPeers)
//					item.AddLog(
//						metamorph_api.Status_ANNOUNCED_TO_NETWORK,
//						"expired",
//						"Re-announced expired tx",
//					)
//				}
//
//				p.retries.AddDuration(time.Since(startTime))
//			}
//		}
//	}
//}

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
		pr.Start = record.StoredAt

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

			if transactionResponse == nil || transactionResponse.GetBlockHeight() <= 0 {
				return
			}

			// we have a mined transaction, let's update the status
			var blockHash *chainhash.Hash
			blockHash, err = chainhash.NewHash(transactionResponse.GetBlockHash())
			if err != nil {
				p.logger.Error("Failed to convert block hash", slog.String("err", err.Error()))
				return
			}

			err = p.SendStatusMinedForTransaction(record.Hash, blockHash, transactionResponse.GetBlockHeight())
			if err != nil {
				p.logger.Error("Failed to update status for mined transaction", slog.String("err", err.Error()))
			}
		}
	})
	if err != nil {
		p.logger.Error("Failed to iterate through stored transactions", slog.String("err", err.Error()))
	}
}

func (p *Processor) SendStatusMinedForTransaction(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	err := p.store.UpdateMined(context.TODO(), hash, blockHash, blockHeight)
	if err != nil {
		return fmt.Errorf("failed to get tx %s from response map", hash.String())
	}
	return nil
}

var statusValueMap = map[metamorph_api.Status]int{
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

func (p *Processor) SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status) error {
	//processorResponse, ok := p.ProcessorResponseMap.Get(hash)
	tx, err := p.store.Get(context.TODO(), hash.CloneBytes())
	if err != nil {
		return err
	}

	// Do not overwrite a higher value status with a lower or equal value status
	if statusValueMap[status] <= statusValueMap[tx.Status] {
		p.logger.Debug("Status not updated for tx", slog.String("status", status.String()), slog.String("previous status", tx.Status.String()), slog.String("hash", hash.String()))
		return nil
	}

	err = p.store.UpdateStatus(context.TODO(), hash, status, "")
	if err != nil {
		return err
	}
	return nil
}

func (p *Processor) ProcessTransaction(ctx context.Context, req *ProcessorRequest) {

	// we need to decouple the Context from the request, so that we don't get cancelled
	// when the request is cancelled
	callerSpan := opentracing.SpanFromContext(ctx)
	newctx := opentracing.ContextWithSpan(context.Background(), callerSpan)
	span, spanCtx := opentracing.StartSpanFromContext(newctx, "Processor:processTransaction")
	defer span.Finish()

	p.queuedCount.Add(1)

	p.logger.Debug("Adding channel", slog.String("hash", req.Data.Hash.String()))

	processorResponse := processor_response.NewProcessorResponseWithChannel(req.Data.Hash, req.ResponseChannel)

	err := p.store.Set(spanCtx, req.Data.Hash[:], req.Data)
	if err != nil {
		p.logger.Error("failed to update status", slog.String("error", err.Error()))
		return
	}

	peers := p.pm.AnnounceTransaction(req.Data.Hash, nil)
	processorResponse.SetPeers(peers)

	peersStr := make([]string, len(peers))
	if len(peers) > 0 {
		for _, peer := range peers {
			peersStr = append(peersStr, peer.String())
		}
	}

	err = p.store.UpdateStatus(spanCtx, req.Data.Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "")
	if err != nil {
		p.logger.Error("failed to update status", slog.String("error", err.Error()))
		return
	}

}
