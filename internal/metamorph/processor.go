package metamorph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

const (
	// maxRetriesDefault number of times we will retry announcing transaction if we haven't seen it on the network
	maxRetriesDefault = 1000
	// length of interval for checking transactions if they are seen on the network
	// if not we resend them again for a few times
	unseenTransactionRebroadcastingInterval    = 60 * time.Second
	seenOnNetworkTransactionRequestingInterval = 3 * time.Minute

	mapExpiryTimeDefault            = 24 * time.Hour
	seenOnNetworkTxTimeDefault      = 3 * 24 * time.Hour
	seenOnNetworkTxTimeUntilDefault = 2 * time.Hour
	LogLevelDefault                 = slog.LevelInfo

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
	ErrPeerManagerNil               = errors.New("peer manager cannot be nil")
	ErrFailedToUnmarshalMessage     = errors.New("failed to unmarshal message")
	ErrFailedToSubscribe            = errors.New("failed to subscribe to topic")
	ErrFailedToStartCollectingStats = errors.New("failed to start collecting stats")
	ErrUnhealthy                    = fmt.Errorf("processor has less than %d healthy peer connections", minimumHealthyConnectionsDefault)
)

type Processor struct {
	store                     store.MetamorphStore
	cacheStore                cache.Store
	hostname                  string
	pm                        p2p.PeerManagerI
	mqClient                  MessageQueue
	logger                    *slog.Logger
	mapExpiryTime             time.Duration
	seenOnNetworkTxTime       time.Duration
	seenOnNetworkTxTimeUntil  time.Duration
	now                       func() time.Time
	stats                     *processorStats
	maxRetries                int
	minimumHealthyConnections int
	callbackSender            CallbackSender

	responseProcessor *ResponseProcessor
	statusMessageCh   chan *TxStatusMessage

	waitGroup *sync.WaitGroup

	statCollectionInterval time.Duration

	cancelAll context.CancelFunc
	ctx       context.Context

	lockTransactionsInterval time.Duration

	minedTxsChan     chan *blocktx_api.TransactionBlock
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
}

type Option func(f *Processor)

type CallbackSender interface {
	SendCallback(ctx context.Context, data *store.Data)
}

func NewProcessor(s store.MetamorphStore, c cache.Store, pm p2p.PeerManagerI, statusMessageChannel chan *TxStatusMessage, opts ...Option) (*Processor, error) {
	if s == nil {
		return nil, ErrStoreNil
	}

	if pm == nil {
		return nil, ErrPeerManagerNil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	p := &Processor{
		store:                     s,
		cacheStore:                c,
		hostname:                  hostname,
		pm:                        pm,
		mapExpiryTime:             mapExpiryTimeDefault,
		seenOnNetworkTxTime:       seenOnNetworkTxTimeDefault,
		seenOnNetworkTxTimeUntil:  seenOnNetworkTxTimeUntilDefault,
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
		serialized := &blocktx_api.TransactionBlock{}
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

	p.pm.Shutdown()

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
	var txsBlocks []*blocktx_api.TransactionBlock
	ticker := time.NewTicker(p.processMinedInterval)
	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case txBlock := <-p.minedTxsChan:
				if txBlock == nil {
					continue
				}

				txsBlocks = append(txsBlocks, txBlock)

				if len(txsBlocks) < p.processMinedBatchSize {
					continue
				}

				p.updateMined(p.ctx, txsBlocks)
				txsBlocks = []*blocktx_api.TransactionBlock{}

				// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
				// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
				ticker.Reset(p.processMinedInterval)

			case <-ticker.C:
				if len(txsBlocks) == 0 {
					continue
				}

				p.updateMined(p.ctx, txsBlocks)
				txsBlocks = []*blocktx_api.TransactionBlock{}

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
		if len(data.Callbacks) > 0 {
			p.callbackSender.SendCallback(ctx, data)
		}
	}
}

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
				callback := store.Callback{
					CallbackURL:   submittedTx.GetCallbackUrl(),
					CallbackToken: submittedTx.GetCallbackToken(),
				}
				sReq := &store.Data{
					Hash:              PtrTo(chainhash.DoubleHashH(submittedTx.GetRawTx())),
					Status:            metamorph_api.Status_STORED,
					Callbacks:         []store.Callback{callback},
					FullStatusUpdates: submittedTx.GetFullStatusUpdates(),
					RawTx:             submittedTx.GetRawTx(),
					StoredAt:          now,
					LastSubmittedAt:   now,
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
				// update status of transaction in storage
				p.storageStatusUpdateCh <- store.UpdateStatus{
					Hash:         *msg.Hash,
					Status:       msg.Status,
					Error:        msg.Err,
					CompetingTxs: msg.CompetingTxs,
				}

				// if we receive new update check if we have client connection waiting for status and send it
				p.responseProcessor.UpdateStatus(msg.Hash, StatusAndError{
					Hash:         msg.Hash,
					Status:       msg.Status,
					Err:          msg.Err,
					CompetingTxs: msg.CompetingTxs,
				})
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
					p.logger.Error("failed to update status", slog.String("err", err.Error()))
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

	for _, data := range updatedData {
		p.logger.Debug("Status updated for tx", slog.String("status", data.Status.String()), slog.String("hash", data.Hash.String()))

		sendCallback := false
		if data.FullStatusUpdates {
			sendCallback = data.Status >= metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL
		} else {
			sendCallback = data.Status >= metamorph_api.Status_REJECTED
		}

		if sendCallback && len(data.Callbacks) > 0 {
			p.callbackSender.SendCallback(ctx, data)
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
				getSeenOnNetworkSince := p.now().Add(-1 * p.seenOnNetworkTxTime)
				getSeenOnNetworkUntil := p.now().Add(-1 * p.seenOnNetworkTxTimeUntil)
				var offset int64
				var totalSeenOnNetworkTxs int

				for {
					seenOnNetworkTxs, err := p.store.GetSeenOnNetwork(ctx, getSeenOnNetworkSince, getSeenOnNetworkUntil, loadSeenOnNetworkLimit, offset)
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
						// by requesting tx, blocktx checks if it has the transaction mined in the database and sends it back
						if err = p.mqClient.Publish(ctx, RequestTxTopic, tx.Hash[:]); err != nil {
							p.logger.Error("failed to request tx from blocktx", slog.String("hash", tx.Hash.String()), slog.String("err", err.Error()))
						}
					}
				}

				if totalSeenOnNetworkTxs > 0 {
					p.logger.Info("SEEN_ON_NETWORK txs being requested", slog.Int("number", totalSeenOnNetworkTxs))
				}

				tracing.EndTracing(span, nil)
			}
		}
	}()
}

func (p *Processor) StartProcessExpiredTransactions() {
	ticker := time.NewTicker(p.processExpiredTxsInterval)
	p.waitGroup.Add(1)

	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C: // Periodically read unmined transactions from database and announce them again
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

						// mark that we retried processing this transaction once more
						if err = p.store.IncrementRetries(ctx, tx.Hash); err != nil {
							p.logger.Error("Failed to increment retries in database", slog.String("err", err.Error()))
						}

						// every second time request tx, every other time announce tx
						if tx.Retries%2 != 0 {
							// Send GETDATA to peers to see if they have it
							p.logger.Debug("Re-getting expired tx", slog.String("hash", tx.Hash.String()))
							p.pm.RequestTransaction(tx.Hash)
							requested++
							continue
						}

						p.logger.Debug("Re-announcing expired tx", slog.String("hash", tx.Hash.String()))
						peers := p.pm.AnnounceTransaction(tx.Hash, nil)
						if len(peers) == 0 {
							p.logger.Warn("transaction was not announced to any peer during rebroadcast", slog.String("hash", tx.Hash.String()))
							continue
						}
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

// GetPeers returns a list of connected and a list of disconnected peers
func (p *Processor) GetPeers() []p2p.PeerI {
	return p.pm.GetPeers()
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

	// register transaction in blocktx using message queue
	if err = p.mqClient.Publish(ctx, RegisterTxTopic, req.Data.Hash[:]); err != nil {
		p.logger.Error("failed to register tx in blocktx", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
	}

	// broadcast that transaction is stored to client
	statusResponse.UpdateStatus(StatusAndError{
		Status: metamorph_api.Status_STORED,
	})

	// Send GETDATA to peers to see if they have it
	ctx, requestTransactionSpan := tracing.StartTracing(ctx, "RequestTransaction", p.tracingEnabled, p.tracingAttributes...)
	p.pm.RequestTransaction(req.Data.Hash)
	tracing.EndTracing(requestTransactionSpan, nil)

	// Announce transaction to network peers
	p.logger.Debug("announcing transaction", slog.String("hash", req.Data.Hash.String()))
	ctx, announceTransactionSpan := tracing.StartTracing(ctx, "AnnounceTransaction", p.tracingEnabled, p.tracingAttributes...)
	peers := p.pm.AnnounceTransaction(req.Data.Hash, nil)
	tracing.EndTracing(announceTransactionSpan, nil)
	if len(peers) == 0 {
		p.logger.Warn("transaction was not announced to any peer", slog.String("hash", req.Data.Hash.String()))
		return
	}

	// update status in response
	statusResponse.UpdateStatus(StatusAndError{
		Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
	})

	// update status in storage
	p.storageStatusUpdateCh <- store.UpdateStatus{
		Hash:   *req.Data.Hash,
		Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
	}

	// Add this transaction to the map of transactions that client is listening to with open connection
	_, responseProcessorAddSpan := tracing.StartTracing(ctx, "responseProcessor.Add", p.tracingEnabled, p.tracingAttributes...)
	p.responseProcessor.Add(statusResponse)
	tracing.EndTracing(responseProcessorAddSpan, nil)
}

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
		// register transaction in blocktx using message queue
		err = p.mqClient.Publish(ctx, RegisterTxTopic, data.Hash[:])
		if err != nil {
			p.logger.Error("Failed to register tx in blocktx", slog.String("hash", data.Hash.String()), slog.String("err", err.Error()))
		}

		// Announce transaction to network and save peers
		peers := p.pm.AnnounceTransaction(data.Hash, nil)
		if len(peers) == 0 {
			p.logger.Warn("transaction was not announced to any peer", slog.String("hash", data.Hash.String()))
			continue
		}

		// update status in storage
		p.storageStatusUpdateCh <- store.UpdateStatus{
			Hash:   *data.Hash,
			Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		}
	}
}

func (p *Processor) Health() error {
	healthyConnections := 0

	for _, peer := range p.pm.GetPeers() {
		if peer.Connected() && peer.IsHealthy() {
			healthyConnections++
		}
	}

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
	if reqData.Callbacks == nil {
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
