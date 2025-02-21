package metamorph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	// maxRetriesDefault number of times we will retry announcing transaction if we haven't seen it on the network
	maxRetriesDefault = 1000
	// length of interval for checking transactions if they are seen on the network
	// if not we resend them again for a few times
	unseenTransactionRebroadcastingInterval    = 60 * time.Second
	seenOnNetworkTransactionRequestingInterval = 3 * time.Minute

	mapExpiryTimeDefault       = 24 * time.Hour
	recheckSeenFromAgo         = 24 * time.Hour
	recheckSeenUntilAgoDefault = 1 * time.Hour
	LogLevelDefault            = slog.LevelInfo

	txCacheTTL = 10 * time.Minute

	loadUnminedLimit                 = int64(5000)
	loadSeenOnNetworkLimit           = int64(5000)
	minimumHealthyConnectionsDefault = 2

	processStatusUpdatesIntervalDefault  = 500 * time.Millisecond
	processStatusUpdatesBatchSizeDefault = 1000

	processTransactionsBatchSizeDefault = 200
	processTransactionsIntervalDefault  = 1 * time.Second

	processMinedBatchSizeDefault = 200
	processMinedIntervalDefault  = 1 * time.Second
)

var (
	ErrStoreNil                     = errors.New("store cannot be nil")
	ErrPeerMessengerNil             = errors.New("p2p messenger cannot be nil")
	ErrFailedToUnmarshalMessage     = errors.New("failed to unmarshal message")
	ErrFailedToSubscribe            = errors.New("failed to subscribe to topic")
	ErrFailedToStartCollectingStats = errors.New("failed to start collecting stats")
	ErrUnhealthy                    = fmt.Errorf("processor has less than %d healthy peer connections", minimumHealthyConnectionsDefault)
)

type Processor struct {
	store                     store.MetamorphStore
	cacheStore                cache.Store
	hostname                  string
	bcMediator                *bcnet.Mediator
	mqClient                  MessageQueue
	logger                    *slog.Logger
	mapExpiryTime             time.Duration
	recheckSeenFromAgo        time.Duration
	recheckSeenUntilAgo       time.Duration
	now                       func() time.Time
	stats                     *processorStats
	maxRetries                int
	minimumHealthyConnections int
	callbackSender            CallbackSender

	responseProcessor *ResponseProcessor
	statusMessageCh   chan *metamorph_p2p.TxStatusMessage

	waitGroup *sync.WaitGroup

	statCollectionInterval time.Duration

	cancelAll context.CancelFunc
	ctx       context.Context

	lockTransactionsInterval time.Duration

	minedTxsChan     chan *blocktx_api.TransactionBlocks
	submittedTxsChan chan *metamorph_api.TransactionRequest

	storageStatusUpdateCh         chan store.UpdateStatus
	processStatusUpdatesInterval  time.Duration
	processStatusUpdatesBatchSize int

	processExpiredTxsInterval       time.Duration
	processSeenOnNetworkTxsInterval time.Duration

	processTransactionsInterval  time.Duration
	processTransactionsBatchSize int

	processMinedInterval  time.Duration
	processMinedBatchSize int

	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue

	blocktxClient blocktx.Client
}

type Option func(f *Processor)

type CallbackSender interface {
	SendCallback(ctx context.Context, data *store.Data)
}

func NewProcessor(s store.MetamorphStore, c cache.Store, bcMediator *bcnet.Mediator, statusMessageChannel chan *metamorph_p2p.TxStatusMessage, opts ...Option) (*Processor, error) {
	if s == nil {
		return nil, ErrStoreNil
	}

	if bcMediator == nil {
		return nil, ErrPeerMessengerNil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	p := &Processor{
		store:                     s,
		cacheStore:                c,
		hostname:                  hostname,
		bcMediator:                bcMediator,
		mapExpiryTime:             mapExpiryTimeDefault,
		recheckSeenFromAgo:        recheckSeenFromAgo,
		recheckSeenUntilAgo:       recheckSeenUntilAgoDefault,
		now:                       time.Now,
		maxRetries:                maxRetriesDefault,
		minimumHealthyConnections: minimumHealthyConnectionsDefault,

		responseProcessor: NewResponseProcessor(),
		statusMessageCh:   statusMessageChannel,

		processExpiredTxsInterval:       unseenTransactionRebroadcastingInterval,
		processSeenOnNetworkTxsInterval: seenOnNetworkTransactionRequestingInterval,
		lockTransactionsInterval:        unseenTransactionRebroadcastingInterval,

		processStatusUpdatesInterval:  processStatusUpdatesIntervalDefault,
		processStatusUpdatesBatchSize: processStatusUpdatesBatchSizeDefault,
		storageStatusUpdateCh:         make(chan store.UpdateStatus, processStatusUpdatesBatchSizeDefault),
		stats:                         newProcessorStats(),
		waitGroup:                     &sync.WaitGroup{},

		statCollectionInterval:       statCollectionIntervalDefault,
		processTransactionsInterval:  processTransactionsIntervalDefault,
		processTransactionsBatchSize: processTransactionsBatchSizeDefault,

		processMinedInterval:  processMinedIntervalDefault,
		processMinedBatchSize: processMinedBatchSizeDefault,
	}

	p.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault})).With(slog.String("service", "mtm"))

	// apply options to processor
	for _, opt := range opts {
		opt(p)
	}

	p.logger.Info("Starting processor", slog.String("cacheExpiryTime", p.mapExpiryTime.String()))

	ctx, cancelAll := context.WithCancel(context.Background())
	p.cancelAll = cancelAll
	p.ctx = ctx

	err = newPrometheusCollector(p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Processor) Start(statsEnabled bool) error {
	err := p.mqClient.Subscribe(MinedTxsTopic, func(msg []byte) error {
		serialized := &blocktx_api.TransactionBlocks{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf("subscribed on %s topic", MinedTxsTopic), err)
		}

		p.minedTxsChan <- serialized
		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf("to %s topic", MinedTxsTopic), err)
	}

	err = p.mqClient.Subscribe(SubmitTxTopic, func(msg []byte) error {
		serialized := &metamorph_api.TransactionRequest{}
		err = proto.Unmarshal(msg, serialized)
		if err != nil {
			return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf("subscribed on %s topic", SubmitTxTopic), err)
		}

		p.submittedTxsChan <- serialized
		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf("to %s topic", SubmitTxTopic), err)
	}

	p.StartLockTransactions()
	time.Sleep(200 * time.Millisecond) // wait a short time so that process expired transactions will start shortly after lock transactions go routine

	p.StartProcessExpiredTransactions()
	p.StartRequestingSeenOnNetworkTxs()
	p.StartProcessStatusUpdatesInStorage()
	p.StartProcessMinedCallbacks()
	if statsEnabled {
		err = p.StartCollectStats()
		if err != nil {
			return errors.Join(ErrFailedToStartCollectingStats, err)
		}
	}
	p.StartSendStatusUpdate()
	p.StartProcessSubmittedTxs()

	return nil
}

// Shutdown closes all channels and goroutines gracefully
func (p *Processor) Shutdown() {
	p.logger.Info("Shutting down processor")

	err := p.unlockRecords()
	if err != nil {
		p.logger.Error("Failed to unlock all hashes", slog.String("err", err.Error()))
	}

	if p.cancelAll != nil {
		p.cancelAll()
	}

	p.waitGroup.Wait()
}

func (p *Processor) unlockRecords() error {
	unlockedItems, err := p.store.SetUnlockedByName(context.Background(), p.hostname)
	if err != nil {
		return err
	}
	p.logger.Info("unlocked items", slog.Int64("number", unlockedItems))

	return nil
}

func (p *Processor) StartProcessMinedCallbacks() {
	p.waitGroup.Add(1)
	var txsBlocksBuffer []*blocktx_api.TransactionBlock
	ticker := time.NewTicker(p.processMinedInterval)
	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case msg := <-p.minedTxsChan:
				if msg == nil {
					continue
				}

				if len(msg.TransactionBlocks) >= p.processMinedBatchSize {
					p.updateMined(p.ctx, msg.TransactionBlocks)
					continue
				}

				txsBlocksBuffer = append(txsBlocksBuffer, msg.TransactionBlocks...)

				if len(txsBlocksBuffer) < p.processMinedBatchSize {
					continue
				}

				p.updateMined(p.ctx, txsBlocksBuffer)
				txsBlocksBuffer = []*blocktx_api.TransactionBlock{}

				// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
				// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
				ticker.Reset(p.processMinedInterval)

			case <-ticker.C:
				if len(txsBlocksBuffer) == 0 {
					continue
				}

				p.updateMined(p.ctx, txsBlocksBuffer)
				txsBlocksBuffer = []*blocktx_api.TransactionBlock{}

				// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
				// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
				ticker.Reset(p.processMinedInterval)
			}
		}
	}()
}

func (p *Processor) updateMined(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "updateMined", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	updatedData, err := p.store.UpdateMined(ctx, txsBlocks)
	if err != nil {
		p.logger.Error("failed to register transactions", slog.String("err", err.Error()))
		return
	}

	for _, data := range updatedData {
		// if we have a pending request with given transaction hash, provide mined status
		p.responseProcessor.UpdateStatus(data.Hash, StatusAndError{
			Hash:   data.Hash,
			Status: metamorph_api.Status_MINED,
		})

		if len(data.Callbacks) > 0 {
			requests := toSendRequest(data, p.now())
			for _, request := range requests {
				err = p.mqClient.PublishMarshal(ctx, CallbackTopic, request)
				if err != nil {
					p.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}

		p.delTxFromCache(data.Hash)
	}
}

// StartProcessSubmittedTxs starts processing txs submitted to message queue
func (p *Processor) StartProcessSubmittedTxs() {
	p.waitGroup.Add(1)
	ticker := time.NewTicker(p.processTransactionsInterval)
	go func() {
		defer p.waitGroup.Done()

		reqs := make([]*store.Data, 0, p.processTransactionsBatchSize)
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				if len(reqs) > 0 {
					p.ProcessTransactions(p.ctx, reqs)
					reqs = make([]*store.Data, 0, p.processTransactionsBatchSize)

					// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
					// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
					ticker.Reset(p.processTransactionsInterval)
				}
			case submittedTx := <-p.submittedTxsChan:
				if submittedTx == nil {
					continue
				}
				now := p.now()
				sReq := &store.Data{
					Hash:              PtrTo(chainhash.DoubleHashH(submittedTx.GetRawTx())),
					Status:            metamorph_api.Status_STORED,
					FullStatusUpdates: submittedTx.GetFullStatusUpdates(),
					RawTx:             submittedTx.GetRawTx(),
					Callbacks:         []store.Callback{},
					StoredAt:          now,
					LastSubmittedAt:   now,
				}

				if submittedTx.GetCallbackUrl() != "" || submittedTx.GetCallbackToken() != "" {
					sReq.Callbacks = []store.Callback{
						{
							CallbackURL:   submittedTx.GetCallbackUrl(),
							CallbackToken: submittedTx.GetCallbackToken(),
						},
					}
				}

				reqs = append(reqs, sReq)
				if len(reqs) >= p.processTransactionsBatchSize {
					p.ProcessTransactions(p.ctx, reqs)
					reqs = make([]*store.Data, 0, p.processTransactionsBatchSize)

					// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
					// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
					ticker.Reset(p.processTransactionsInterval)
				}
			}
		}
	}()
}

func (p *Processor) StartSendStatusUpdate() {
	p.waitGroup.Add(1)
	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return

			case msg := <-p.statusMessageCh:
				// if we receive new update check if we have client connection waiting for status and send it
				found := p.responseProcessor.UpdateStatus(msg.Hash, StatusAndError{
					Hash:         msg.Hash,
					Status:       msg.Status,
					Err:          msg.Err,
					CompetingTxs: msg.CompetingTxs,
				})

				if !found {
					found = p.txFoundInCache(msg.Hash)
				}

				if !found {
					continue
				}

				p.logger.Debug("Status update received", slog.String("hash", msg.Hash.String()), slog.String("status", msg.Status.String()))

				// update status of transaction in storage
				p.storageStatusUpdateCh <- store.UpdateStatus{
					Hash:         *msg.Hash,
					Status:       msg.Status,
					Error:        msg.Err,
					CompetingTxs: msg.CompetingTxs,
					Timestamp:    msg.Start,
				}

				// if tx is rejected, we don't expect any more status updates on this channel - remove from cache
				if msg.Status == metamorph_api.Status_REJECTED {
					p.delTxFromCache(msg.Hash)
				}
			}
		}
	}()
}

func (p *Processor) StartProcessStatusUpdatesInStorage() {
	ticker := time.NewTicker(p.processStatusUpdatesInterval)
	p.waitGroup.Add(1)

	ctx := p.ctx

	go func() {
		defer p.waitGroup.Done()

		for {
			select {
			case <-p.ctx.Done():
				return
			case statusUpdate := <-p.storageStatusUpdateCh:
				// Ensure no duplicate statuses
				err := p.updateStatusMap(statusUpdate)
				if err != nil {
					p.logger.Error("failed to update status", slog.String("err", err.Error()), slog.String("hash", statusUpdate.Hash.String()))
					return
				}

				statusUpdateCount, err := p.getStatusUpdateCount()
				if err != nil {
					p.logger.Error("failed to get status update count", slog.String("err", err.Error()))
					return
				}

				if statusUpdateCount >= p.processStatusUpdatesBatchSize {
					err := p.checkAndUpdate(ctx)
					if err != nil {
						p.logger.Error("failed to check and update statuses", slog.String("err", err.Error()))
					}

					// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
					// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
					ticker.Reset(p.processStatusUpdatesInterval)
				}
			case <-ticker.C:
				statusUpdateCount, err := p.getStatusUpdateCount()
				if err != nil {
					p.logger.Error("failed to get status update count", slog.String("err", err.Error()))
					return
				}

				if statusUpdateCount > 0 {
					err := p.checkAndUpdate(ctx)
					if err != nil {
						p.logger.Error("failed to check and update statuses", slog.String("err", err.Error()))
					}

					// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
					// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
					ticker.Reset(p.processStatusUpdatesInterval)
				}
			}
		}
	}()
}

func (p *Processor) checkAndUpdate(ctx context.Context) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "checkAndUpdate", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	statusUpdatesMap, err := p.getAndDeleteAllTransactionStatuses()
	if err != nil {
		return err
	}

	if len(statusUpdatesMap) == 0 {
		return nil
	}

	statusUpdates := make([]store.UpdateStatus, 0, len(statusUpdatesMap))
	doubleSpendUpdates := make([]store.UpdateStatus, 0)

	for _, status := range statusUpdatesMap {
		if len(status.CompetingTxs) > 0 {
			doubleSpendUpdates = append(doubleSpendUpdates, status)
		} else {
			statusUpdates = append(statusUpdates, status)
		}
	}

	err = p.statusUpdateWithCallback(ctx, statusUpdates, doubleSpendUpdates)
	if err != nil {
		p.logger.Error("failed to bulk update statuses", slog.String("err", err.Error()))
	}

	return nil
}

func (p *Processor) statusUpdateWithCallback(ctx context.Context, statusUpdates, doubleSpendUpdates []store.UpdateStatus) (err error) {
	ctx, span := tracing.StartTracing(ctx, "statusUpdateWithCallback", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var updatedData []*store.Data

	if len(statusUpdates) > 0 {
		updatedData, err = p.store.UpdateStatusBulk(ctx, statusUpdates)
		if err != nil {
			return err
		}
	}

	if len(doubleSpendUpdates) > 0 {
		updatedDoubleSpendData, err := p.store.UpdateDoubleSpend(ctx, doubleSpendUpdates)
		if err != nil {
			return err
		}
		updatedData = append(updatedData, updatedDoubleSpendData...)
	}

	statusHistoryUpdates := filterUpdates(statusUpdates, updatedData)
	_, err = p.store.UpdateStatusHistoryBulk(ctx, statusHistoryUpdates)
	if err != nil {
		p.logger.Error("failed to update status history", slog.String("err", err.Error()))
	}

	for _, data := range updatedData {
		p.logger.Debug("Status updated for tx", slog.String("status", data.Status.String()), slog.String("hash", data.Hash.String()))
		sendCallback := false
		if data.FullStatusUpdates {
			sendCallback = data.Status >= metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL
		} else {
			sendCallback = data.Status >= metamorph_api.Status_REJECTED
		}

		if sendCallback && len(data.Callbacks) > 0 {
			requests := toSendRequest(data, p.now())
			for _, request := range requests {
				err = p.mqClient.PublishMarshal(ctx, CallbackTopic, request)
				if err != nil {
					p.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}
	}
	return nil
}

func (p *Processor) StartLockTransactions() {
	ticker := time.NewTicker(p.lockTransactionsInterval)
	p.waitGroup.Add(1)

	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				expiredSince := p.now().Add(-1 * p.mapExpiryTime)
				err := p.store.SetLocked(p.ctx, expiredSince, loadUnminedLimit)
				if err != nil {
					p.logger.Error("Failed to set transactions locked", slog.String("err", err.Error()))
				}
			}
		}
	}()
}

func (p *Processor) StartRequestingSeenOnNetworkTxs() {
	ticker := time.NewTicker(p.processSeenOnNetworkTxsInterval)
	p.waitGroup.Add(1)

	go func() {
		defer p.waitGroup.Done()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				ctx, span := tracing.StartTracing(p.ctx, "StartRequestingSeenOnNetworkTxs", p.tracingEnabled, p.tracingAttributes...)

				// Periodically read SEEN_ON_NETWORK transactions from database check their status in blocktx
				getSeenOnNetworkFrom := p.now().Add(-1 * p.recheckSeenFromAgo)
				getSeenOnNetworkUntil := p.now().Add(-1 * p.recheckSeenUntilAgo)
				var offset int64
				var totalSeenOnNetworkTxs int

				for {
					seenOnNetworkTxs, err := p.store.GetSeenOnNetwork(ctx, getSeenOnNetworkFrom, getSeenOnNetworkUntil, loadSeenOnNetworkLimit, offset)
					offset += loadSeenOnNetworkLimit
					if err != nil {
						p.logger.Error("Failed to get SeenOnNetwork transactions", slog.String("err", err.Error()))
						break
					}

					if len(seenOnNetworkTxs) == 0 {
						break
					}

					totalSeenOnNetworkTxs += len(seenOnNetworkTxs)

					for _, tx := range seenOnNetworkTxs {
						err = p.registerTransaction(ctx, tx.Hash)
						if err != nil {
							p.logger.Error("Failed to register tx in blocktx", slog.String("hash", tx.Hash.String()), slog.String("err", err.Error()))
						}
					}
				}

				if totalSeenOnNetworkTxs > 0 {
					p.logger.Info("Transactions seen on network re-registered", slog.Int("number", totalSeenOnNetworkTxs))
				}

				tracing.EndTracing(span, nil)
			}
		}
	}()
}

// StartProcessExpiredTransactions periodically reads transactions with status lower than SEEN_ON_NETWORK from database and announce them again
func (p *Processor) StartProcessExpiredTransactions() {
	p.waitGroup.Add(1)

	go func() {
		defer p.waitGroup.Done()

		ticker := time.NewTicker(p.processExpiredTxsInterval)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				ctx, span := tracing.StartTracing(p.ctx, "StartProcessExpiredTransactions", p.tracingEnabled, p.tracingAttributes...)

				// define from what point in time we are interested in unmined transactions
				getUnminedSince := p.now().Add(-1 * p.mapExpiryTime)
				var offset int64

				requested := 0
				announced := 0
				for {
					// get all transactions since then chunk by chunk
					unminedTxs, err := p.store.GetUnmined(ctx, getUnminedSince, loadUnminedLimit, offset)
					if err != nil {
						p.logger.Error("Failed to get unmined transactions", slog.String("err", err.Error()))
						break
					}

					offset += loadUnminedLimit
					if len(unminedTxs) == 0 {
						break
					}

					for _, tx := range unminedTxs {
						if tx.Retries > p.maxRetries {
							continue
						}

						// save the tx to cache again, in case it was removed or expired
						err := p.saveTxToCache(tx.Hash)
						if err != nil {
							p.logger.Error("Failed to store tx in cache", slog.String("hash", tx.Hash.String()), slog.String("err", err.Error()))
							continue
						}

						// mark that we retried processing this transaction once more
						if err = p.store.IncrementRetries(ctx, tx.Hash); err != nil {
							p.logger.Error("Failed to increment retries in database", slog.String("err", err.Error()))
						}

						// every second time request tx, every other time announce tx
						if tx.Retries%2 != 0 {
							// Send GETDATA to peers to see if they have it
							p.logger.Debug("Re-getting expired tx", slog.String("hash", tx.Hash.String()))
							p.bcMediator.AskForTxAsync(ctx, tx)
							requested++
							continue
						}

						p.logger.Debug("Re-announcing expired tx", slog.String("hash", tx.Hash.String()))
						p.bcMediator.AnnounceTxAsync(ctx, tx)
						announced++
					}
				}

				if announced > 0 || requested > 0 {
					p.logger.Info("Retried unmined transactions", slog.Int("announced", announced), slog.Int("requested", requested), slog.Time("since", getUnminedSince))
				}

				tracing.EndTracing(span, nil)
			}
		}
	}()
}

// GetPeers returns a list of connected and disconnected peers
func (p *Processor) GetPeers() []p2p.PeerI {
	return p.bcMediator.GetPeers()
}

func (p *Processor) registerTransaction(ctx context.Context, hash *chainhash.Hash) error {
	err := p.blocktxClient.RegisterTransaction(ctx, hash[:])
	if err == nil {
		return nil
	}

	p.logger.Warn("Register transaction call failed", slog.String("err", err.Error()))

	err = p.mqClient.Publish(ctx, RegisterTxTopic, hash[:])
	if err != nil {
		return fmt.Errorf("failed to publish hash on topic %s: %w", RegisterTxTopic, err)
	}

	return nil
}

func (p *Processor) ProcessTransaction(ctx context.Context, req *ProcessorRequest) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "ProcessTransaction", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	statusResponse := NewStatusResponse(ctx, req.Data.Hash, req.ResponseChannel)

	// check if tx already stored, return it
	data, err := p.store.Get(p.ctx, req.Data.Hash[:])
	if err == nil {
		//	When transaction is re-submitted we update last_submitted_at with now()
		//	to make sure it will be loaded and re-broadcast if needed.
		addNewCallback(data, req.Data)
		err = p.storeData(p.ctx, data)
		if err != nil {
			p.logger.Error("Failed to update data", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
		}

		var rejectErr error
		if data.RejectReason != "" {
			rejectErr = errors.New(data.RejectReason)
		}

		// notify the client instantly and return without waiting for any specific status
		statusResponse.UpdateStatus(StatusAndError{
			Status:       data.Status,
			Err:          rejectErr,
			CompetingTxs: data.CompetingTxs,
		})
		return
	}

	if !errors.Is(err, store.ErrNotFound) {
		statusResponse.UpdateStatus(StatusAndError{
			Status: metamorph_api.Status_RECEIVED,
			Err:    err,
		})
		return
	}

	// store in database
	// set tx status to Stored
	sh := &store.Status{Status: req.Data.Status, Timestamp: p.now()}
	req.Data.StatusHistory = append(req.Data.StatusHistory, sh)
	req.Data.Status = metamorph_api.Status_STORED

	if err = p.storeData(ctx, req.Data); err != nil {
		// issue with the store itself
		// notify the client instantly and return
		p.logger.Error("Failed to store transaction", slog.String("hash", data.Hash.String()), slog.String("err", err.Error()))
		statusResponse.UpdateStatus(StatusAndError{
			Status: metamorph_api.Status_RECEIVED,
			Err:    err,
		})
		return
	}

	// broadcast that transaction is stored to client
	statusResponse.UpdateStatus(StatusAndError{
		Status: metamorph_api.Status_STORED,
	})

	// register transaction in blocktx using message queue
	err = p.registerTransaction(ctx, req.Data.Hash)
	if err != nil {
		p.logger.Error("Failed to register tx in blocktx", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
	}

	// Add this transaction to the map of transactions that client is listening to with open connection
	_, responseProcessorAddSpan := tracing.StartTracing(ctx, "responseProcessor.Add", p.tracingEnabled, p.tracingAttributes...)
	p.responseProcessor.Add(statusResponse)
	tracing.EndTracing(responseProcessorAddSpan, nil)

	// Add this transaction to cache
	err = p.saveTxToCache(statusResponse.Hash)
	if err != nil {
		p.logger.Error("failed to store tx in cache", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
		// don't return here, because the transaction will try to be added to cache again when re-broadcasting unmined txs
	}

	// ask network about the tx to see if they have it
	p.bcMediator.AskForTxAsync(ctx, req.Data)
	p.bcMediator.AnnounceTxAsync(ctx, req.Data)

	// update status in response
	statusResponse.UpdateStatus(StatusAndError{
		Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
	})

	// update status in storage
	p.storageStatusUpdateCh <- store.UpdateStatus{
		Hash:      *req.Data.Hash,
		Status:    metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		Timestamp: p.now(),
	}
}

// ProcessTransactions processes txs submitted to message queue
func (p *Processor) ProcessTransactions(ctx context.Context, sReq []*store.Data) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "ProcessTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// store in database
	err = p.store.SetBulk(ctx, sReq)
	if err != nil {
		p.logger.Error("Failed to bulk store txs", slog.Int("number", len(sReq)), slog.String("err", err.Error()))
		return
	}

	for _, data := range sReq {
		err = p.saveTxToCache(data.Hash)
		if err != nil {
			p.logger.Error("Failed to save tx in cache", slog.String("hash", data.Hash.String()), slog.String("err", err.Error()))
		}

		// register transaction in blocktx using message queue
		err = p.registerTransaction(ctx, data.Hash)
		if err != nil {
			p.logger.Error("Failed to register tx in blocktx", slog.String("hash", data.Hash.String()), slog.String("err", err.Error()))
		}

		p.bcMediator.AnnounceTxAsync(ctx, data)

		// update status in storage
		p.storageStatusUpdateCh <- store.UpdateStatus{
			Hash:      *data.Hash,
			Status:    metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			Timestamp: p.now(),
		}
	}
}

func (p *Processor) Health() error {
	healthyConnections := int(p.bcMediator.CountConnectedPeers()) // #nosec G115

	if healthyConnections < p.minimumHealthyConnections {
		p.logger.Warn("Less than expected healthy peers", slog.Int("connections", healthyConnections))
		return ErrUnhealthy
	}

	return nil
}

func (p *Processor) storeData(ctx context.Context, data *store.Data) error {
	data.LastSubmittedAt = p.now()
	return p.store.Set(ctx, data)
}

func addNewCallback(data, reqData *store.Data) {
	if len(reqData.Callbacks) == 0 {
		return
	}
	reqCallback := reqData.Callbacks[0]
	if reqCallback.CallbackURL != "" && !callbackExists(reqCallback, data) {
		data.Callbacks = append(data.Callbacks, reqCallback)
	}
}

func callbackExists(callback store.Callback, data *store.Data) bool {
	for _, c := range data.Callbacks {
		if c == callback {
			return true
		}
	}
	return false
}

func (p *Processor) saveTxToCache(hash *chainhash.Hash) error {
	return p.cacheStore.Set(hash.String(), []byte("1"), txCacheTTL)
}

func (p *Processor) txFoundInCache(hash *chainhash.Hash) (found bool) {
	value, err := p.cacheStore.Get(hash.String())
	if err != nil && !errors.Is(err, cache.ErrCacheNotFound) {
		p.logger.Error("count not get the transaction from hash", slog.String("hash", hash.String()), slog.String("err", err.Error()))
		// respond as if the tx is found just in case this transaction is registered
		return true
	}

	return value != nil
}

func (p *Processor) delTxFromCache(hash *chainhash.Hash) {
	err := p.cacheStore.Del(hash.String())
	if err != nil && !errors.Is(err, cache.ErrCacheNotFound) {
		p.logger.Error("unable to delete transaction from cache", slog.String("hash", hash.String()), slog.String("err", err.Error()))
	}
}
