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
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	// maxRetriesDefault number of times we will retry announcing transaction if we haven't seen it on the network
	maxRetriesDefault = 1000
	// length of interval for checking transactions if they are seen on the network
	// if not we resend them again for a few times
	rebroadcastUnseenIntervalDefault = 60 * time.Second
	lockTransactionsIntervalDefault  = 60 * time.Second
	reRegisterSeenIntervalDefault    = 3 * time.Minute

	checkUnconfirmedSeenIntervalDefault = 5 * time.Minute
	rebroadcastExpirationDefault        = 24 * time.Hour
	reAnnounceSeenDefault               = 1 * time.Hour
	reAnnounceSeenIntervalDefault       = 5 * time.Minute
	logLevelDefault                     = slog.LevelInfo

	loadLimit                        = int64(50)
	minimumHealthyConnectionsDefault = 2

	statusUpdatesIntervalDefault  = 500 * time.Millisecond
	statusUpdatesBatchSizeDefault = 1000

	processTransactionsBatchSizeDefault = 200
	processTransactionsIntervalDefault  = 1 * time.Second
	registerBatchSizeDefault            = 50
	processMinedBatchSizeDefault        = 200
	processMinedIntervalDefault         = 1 * time.Second
)

var (
	ErrStoreNil                     = errors.New("store cannot be nil")
	ErrPeerMessengerNil             = errors.New("p2p messenger cannot be nil")
	ErrFailedToUnmarshalMessage     = errors.New("failed to unmarshal message")
	ErrFailedToSubscribe            = errors.New("failed to subscribe to topic")
	ErrFailedToStartCollectingStats = errors.New("failed to start collecting stats")
	ErrUnhealthy                    = errors.New("processor has less than minimum healthy peer connections")
)

type Processor struct {
	store                     store.MetamorphStore
	cacheStore                cache.Store
	hostname                  string
	bcMediator                Mediator
	mqClient                  mq.MessageQueueClient
	logger                    *slog.Logger
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
	submittedTxsChan chan *metamorph_api.PostTransactionRequest

	storageStatusUpdateCh  chan store.UpdateStatus
	statusUpdatesInterval  time.Duration
	statusUpdatesBatchSize int

	reRegisterSeen         time.Duration
	reRegisterSeenInterval time.Duration

	reAnnounceSeen         time.Duration
	reAnnounceSeenInterval time.Duration

	checkUnconfirmedSeenInterval time.Duration

	reAnnounceUnseenInterval time.Duration

	rebroadcastExpiration time.Duration

	processTransactionsInterval  time.Duration
	processTransactionsBatchSize int

	processMinedInterval  time.Duration
	processMinedBatchSize int
	registerBatchSize     int

	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue

	blocktxClient blocktx.Client
}

type Option func(f *Processor)

type CallbackSender interface {
	SendCallback(ctx context.Context, data *store.Data)
}

type Mediator interface {
	AskForTxAsync(ctx context.Context, tx *store.Data)
	AnnounceTxAsync(ctx context.Context, tx *store.Data)
	GetPeers() []p2p.PeerI
	CountConnectedPeers() uint
}

func NewProcessor(s store.MetamorphStore, c cache.Store, bcMediator Mediator, statusMessageChannel chan *metamorph_p2p.TxStatusMessage, opts ...Option) (*Processor, error) {
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
		rebroadcastExpiration:     rebroadcastExpirationDefault,
		maxRetries:                maxRetriesDefault,
		minimumHealthyConnections: minimumHealthyConnectionsDefault,
		now:                       time.Now,

		responseProcessor: NewResponseProcessor(),
		statusMessageCh:   statusMessageChannel,

		reAnnounceSeen:               reAnnounceSeenDefault,
		reAnnounceSeenInterval:       reAnnounceSeenIntervalDefault,
		reAnnounceUnseenInterval:     rebroadcastUnseenIntervalDefault,
		reRegisterSeenInterval:       reRegisterSeenIntervalDefault,
		lockTransactionsInterval:     lockTransactionsIntervalDefault,
		checkUnconfirmedSeenInterval: checkUnconfirmedSeenIntervalDefault,
		statusUpdatesInterval:        statusUpdatesIntervalDefault,
		statusUpdatesBatchSize:       statusUpdatesBatchSizeDefault,
		storageStatusUpdateCh:        make(chan store.UpdateStatus, statusUpdatesBatchSizeDefault),
		stats:                        newProcessorStats(),
		waitGroup:                    &sync.WaitGroup{},
		registerBatchSize:            registerBatchSizeDefault,
		statCollectionInterval:       statCollectionIntervalDefault,
		processTransactionsInterval:  processTransactionsIntervalDefault,
		processTransactionsBatchSize: processTransactionsBatchSizeDefault,

		processMinedInterval:  processMinedIntervalDefault,
		processMinedBatchSize: processMinedBatchSizeDefault,
	}

	p.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "mtm"))

	// apply options to processor
	for _, opt := range opts {
		opt(p)
	}

	p.logger.Info("Starting processor")

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
	err := p.mqClient.QueueSubscribe(mq.MinedTxsTopic, func(msg []byte) error {
		serialized := &blocktx_api.TransactionBlocks{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf("subscribed on %s topic", mq.MinedTxsTopic), err)
		}

		p.minedTxsChan <- serialized
		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf("to %s topic", mq.MinedTxsTopic), err)
	}

	err = p.mqClient.Consume(mq.SubmitTxTopic, func(msg []byte) error {
		serialized := &metamorph_api.PostTransactionRequest{}
		err = proto.Unmarshal(msg, serialized)
		if err != nil {
			return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf("subscribed on %s topic", mq.SubmitTxTopic), err)
		}

		p.submittedTxsChan <- serialized
		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf("to %s topic", mq.SubmitTxTopic), err)
	}

	p.StartLockTransactions()
	time.Sleep(200 * time.Millisecond) // wait a short time so that process expired transactions will start shortly after lock transactions go routine

	p.StartRoutine(p.reAnnounceUnseenInterval, ReAnnounceUnseen, "ReAnnounceUnseen")
	p.StartRoutine(p.reAnnounceSeenInterval, ReAnnounceSeen, "ReAnnounceSeen")
	p.StartRoutine(p.reRegisterSeenInterval, RegisterSeenTxs, "RegisterSeenTxs")
	p.StartRoutine(p.checkUnconfirmedSeenInterval, RejectUnconfirmedRequested, "RejectUnconfirmedRequested")

	p.StartProcessStatusUpdatesInStorage()
	p.StartProcessMinedCallbacks()
	if statsEnabled {
		err = p.StartCollectStats()
		if err != nil {
			return errors.Join(ErrFailedToStartCollectingStats, err)
		}
	}
	p.StartSendStatusUpdate()
	p.StartProcessSubmitted()

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
	p.logger.Info("unlocked items", slog.Int64("count", unlockedItems))

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
	fmt.Println("shota update mined", len(txsBlocks))
	updatedData, err := p.store.UpdateMined(ctx, txsBlocks)
	if err != nil {
		p.logger.Error("failed to register transactions", slog.String("err", err.Error()))
		return
	}

	p.logger.Info("Updated mined", slog.Int("count", len(txsBlocks)))

	for _, data := range updatedData {
		fmt.Println(data.Hash.String(), "shota update mined data", data.Status)
		// if we have a pending request with given transaction hash, provide mined status
		p.responseProcessor.UpdateStatus(data.Hash, StatusAndError{
			Hash:   data.Hash,
			Status: metamorph_api.Status_MINED,
		})

		if len(data.Callbacks) > 0 {
			requests := toSendRequest(data, p.now())
			for _, request := range requests {
				err = p.mqClient.PublishMarshal(ctx, mq.CallbackTopic, request)
				if err != nil {
					p.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}

		p.delTxFromCache(data.Hash)
	}
}

// StartProcessSubmitted starts processing txs submitted to message queue
func (p *Processor) StartProcessSubmitted() {
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
				if msg.ReceivedRawTx {
					err := p.store.MarkConfirmedRequested(p.ctx, msg.Hash)
					if err != nil {
						p.logger.Error("Failed to mark confirmed requested", slog.String("err", err.Error()))
					}
				}

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
	ticker := time.NewTicker(p.statusUpdatesInterval)
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

				if statusUpdateCount >= p.statusUpdatesBatchSize {
					err := p.checkAndUpdate(ctx)
					if err != nil {
						p.logger.Error("failed to check and update statuses", slog.String("err", err.Error()))
					}

					// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
					// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
					ticker.Reset(p.statusUpdatesInterval)
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
					ticker.Reset(p.statusUpdatesInterval)
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
		updatedData, err = p.store.UpdateStatus(ctx, statusUpdates)
		if err != nil {
			return err
		}
	}

	if len(doubleSpendUpdates) > 0 {
		updatedDoubleSpendData, err := p.store.UpdateDoubleSpend(ctx, doubleSpendUpdates, true)
		if err != nil {
			return err
		}
		updatedData = append(updatedData, updatedDoubleSpendData...)
	}

	statusHistoryUpdates := filterUpdates(statusUpdates, updatedData)
	_, err = p.store.UpdateStatusHistory(ctx, statusHistoryUpdates)
	if err != nil {
		p.logger.Error("failed to update status history", slog.String("err", err.Error()))
	}

	for _, data := range updatedData {
		p.logger.Debug("Status updated for tx", slog.String("status", data.Status.String()), slog.String("hash", data.Hash.String()))
		sendCallback := data.Status >= metamorph_api.Status_REJECTED

		if data.FullStatusUpdates {
			sendCallback = data.Status >= metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL
		}

		if sendCallback && len(data.Callbacks) > 0 {
			requests := toSendRequest(data, p.now())
			for _, request := range requests {
				err = p.mqClient.PublishMarshal(ctx, mq.CallbackTopic, request)
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
				expiredSince := p.now().Add(-1 * p.rebroadcastExpiration)
				err := p.store.SetLocked(p.ctx, expiredSince, loadLimit)
				if err != nil {
					p.logger.Error("Failed to set transactions locked", slog.String("err", err.Error()))
				}
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

	err = p.mqClient.PublishAsync(mq.RegisterTxTopic, hash[:])
	if err != nil {
		return fmt.Errorf("failed to publish hash on topic %s: %w", mq.RegisterTxTopic, err)
	}

	return nil
}

func (p *Processor) registerTransactions(ctx context.Context, data []*store.Data) error {
	txHashesBatch := make([][]byte, 0, len(data))

	for _, hash := range data {
		txHashesBatch = append(txHashesBatch, hash.Hash[:])

		if len(txHashesBatch) >= p.registerBatchSize {
			err := p.registerTransactionsBatch(ctx, txHashesBatch)
			if err != nil {
				p.logger.Error("Failed to register transactions batch", slog.String("err", err.Error()))
			}

			txHashesBatch = txHashesBatch[:0]
		}
	}

	if len(txHashesBatch) > 0 {
		err := p.registerTransactionsBatch(ctx, txHashesBatch)
		if err != nil {
			p.logger.Error("Failed to register transactions batch", slog.String("err", err.Error()))
		}
	}

	return nil
}

func (p *Processor) registerTransactionsBatch(ctx context.Context, txHashes [][]byte) error {
	err := p.blocktxClient.RegisterTransactions(ctx, txHashes)
	if err == nil {
		return nil
	}

	p.logger.Warn("Register transactions call failed", slog.String("err", err.Error()))

	var txs []*blocktx_api.Transaction
	for _, hash := range txHashes {
		txs = append(txs, &blocktx_api.Transaction{Hash: hash[:]})
	}
	txsMsg := &blocktx_api.Transactions{Transactions: txs}

	err = p.mqClient.PublishMarshal(ctx, mq.RegisterTxsTopic, txsMsg)
	if err != nil {
		return fmt.Errorf("failed to publish hash on topic %s: %w", mq.RegisterTxsTopic, err)
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
	sh := &store.StatusWithTimestamp{Status: req.Data.Status, Timestamp: p.now()}
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
		p.logger.Error("Failed to bulk store txs", slog.Int("count", len(sReq)), slog.String("err", err.Error()))
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
		return errors.Join(ErrUnhealthy, fmt.Errorf("minimum healthy connections: %d", p.minimumHealthyConnections))
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
	const txCacheTTL = 10 * time.Minute
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
