package metamorph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"google.golang.org/protobuf/proto"
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

type Processor struct {
	store                     store.MetamorphStore
	hostname                  string
	pm                        p2p.PeerManagerI
	mqClient                  MessageQueueClient
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
	statusMessageCh   chan *PeerTxMessage

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

	announcedTransactions     []AnnouncedTransaction
	announcedTransactionsLock sync.Mutex
}

type AnnouncedTransaction struct {
	second uint64
	hash   *chainhash.Hash
}

type Option func(f *Processor)

type CallbackSender interface {
	SendCallback(logger *slog.Logger, tx *store.StoreData)
	Shutdown(logger *slog.Logger)
}

func NewProcessor(s store.MetamorphStore, pm p2p.PeerManagerI, statusMessageChannel chan *PeerTxMessage, opts ...Option) (*Processor, error) {
	if s == nil {
		return nil, errors.New("store cannot be nil")
	}

	if pm == nil {
		return nil, errors.New("peer manager cannot be nil")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	p := &Processor{
		store:                     s,
		hostname:                  hostname,
		pm:                        pm,
		mapExpiryTime:             mapExpiryTimeDefault,
		seenOnNetworkTxTime:       seenOnNetworkTxTimeDefault,
		seenOnNetworkTxTimeUntil:  seenOnNetworkTxTimeUntilDefault,
		now:                       time.Now,
		maxRetries:                maxRetriesDefault,
		minimumHealthyConnections: minimumHealthyConnectionsDefault,

		responseProcessor:     NewResponseProcessor(),
		statusMessageCh:       statusMessageChannel,
		announcedTransactions: make([]AnnouncedTransaction, 0),

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

func (p *Processor) Start() error {
	err := p.mqClient.Subscribe(MinedTxsTopic, func(msg []byte) error {
		serialized := &blocktx_api.TransactionBlock{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return fmt.Errorf("failed to unmarshal message subscribed on %s topic: %w", MinedTxsTopic, err)
		}

		p.minedTxsChan <- serialized
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", MinedTxsTopic, err)
	}

	err = p.mqClient.Subscribe(SubmitTxTopic, func(msg []byte) error {
		serialized := &metamorph_api.TransactionRequest{}
		err = proto.Unmarshal(msg, serialized)
		if err != nil {
			return fmt.Errorf("failed to unmarshal message subscribed on %s topic: %w", SubmitTxTopic, err)
		}

		p.submittedTxsChan <- serialized
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", SubmitTxTopic, err)
	}

	p.StartLockTransactions()
	time.Sleep(200 * time.Millisecond) // wait a short time so that process expired transactions will start shortly after lock transactions go routine

	p.StartProcessExpiredTransactions()
	p.StartRequestingSeenOnNetworkTxs()
	p.StartProcessStatusUpdatesInStorage()
	p.StartCheckingTransactionsInNetwork()
	p.StartProcessMinedCallbacks()
	err = p.StartCollectStats()
	if err != nil {
		return fmt.Errorf("failed to start collecting stats: %v", err)
	}
	p.StartSendStatusUpdate()
	p.StartProcessSubmittedTxs()

	return nil
}

// Shutdown closes all channels and goroutines gracefully
func (p *Processor) Shutdown() {
	p.logger.Info("Shutting down processor")

	if p.callbackSender != nil {
		p.callbackSender.Shutdown(p.logger)
	}

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

				p.updateMined(txsBlocks)
				txsBlocks = []*blocktx_api.TransactionBlock{}

				// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
				// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
				ticker.Reset(p.processMinedInterval)

			case <-ticker.C:
				if len(txsBlocks) == 0 {
					continue
				}

				p.updateMined(txsBlocks)
				txsBlocks = []*blocktx_api.TransactionBlock{}

				// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
				// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
				ticker.Reset(p.processMinedInterval)
			}
		}
	}()
}

func (p *Processor) updateMined(txsBlocks []*blocktx_api.TransactionBlock) {
	updatedData, err := p.store.UpdateMined(p.ctx, txsBlocks)
	if err != nil {
		p.logger.Error("failed to register transactions", slog.String("err", err.Error()))
		return
	}

	for _, data := range updatedData {
		if len(data.Callbacks) > 0 {
			go p.callbackSender.SendCallback(p.logger, data)
		}
	}
}

func (p *Processor) StartProcessSubmittedTxs() {
	p.waitGroup.Add(1)
	ticker := time.NewTicker(p.processTransactionsInterval)
	go func() {
		defer p.waitGroup.Done()

		reqs := make([]*store.StoreData, 0, p.processTransactionsBatchSize)
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				if len(reqs) > 0 {
					p.ProcessTransactions(reqs)
					reqs = make([]*store.StoreData, 0, p.processTransactionsBatchSize)

					// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
					// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
					ticker.Reset(p.processTransactionsInterval)
				}
			case submittedTx := <-p.submittedTxsChan:
				if submittedTx == nil {
					continue
				}
				now := p.now()
				callback := store.StoreCallback{
					CallbackURL:   submittedTx.GetCallbackUrl(),
					CallbackToken: submittedTx.GetCallbackToken(),
				}
				sReq := &store.StoreData{
					Hash:              PtrTo(chainhash.DoubleHashH(submittedTx.GetRawTx())),
					Status:            metamorph_api.Status_STORED,
					Callbacks:         []store.StoreCallback{callback},
					FullStatusUpdates: submittedTx.GetFullStatusUpdates(),
					RawTx:             submittedTx.GetRawTx(),
					StoredAt:          now,
					LastSubmittedAt:   now,
				}

				reqs = append(reqs, sReq)
				if len(reqs) >= p.processTransactionsBatchSize {
					p.ProcessTransactions(reqs)
					reqs = make([]*store.StoreData, 0, p.processTransactionsBatchSize)

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

				p.logger.Debug("Status reported for tx", slog.String("status", msg.Status.String()), slog.String("hash", msg.Hash.String()))
			}
		}
	}()
}

func (p *Processor) StartCheckingTransactionsInNetwork() {
	p.waitGroup.Add(1)
	ticker := time.NewTicker(100 * time.Millisecond)
	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return

			case <-ticker.C:
				p.announcedTransactionsLock.Lock()
				for k := 0; k < len(p.announcedTransactions); k++ {
					if p.announcedTransactions[k].second < uint64(time.Now().Unix())-2 {
						p.logger.Info("requested transaction", slog.String("hash", p.announcedTransactions[k].hash.String()))
						p.pm.RequestTransaction((*chainhash.Hash)(p.announcedTransactions[k].hash))
						if k == len(p.announcedTransactions)-1 {
							p.announcedTransactions = []AnnouncedTransaction{}
							break
						}
					} else {
						p.announcedTransactions = p.announcedTransactions[k:]
						break
					}
				}
				p.announcedTransactionsLock.Unlock()
			}
		}
	}()
}

func (p *Processor) StartProcessStatusUpdatesInStorage() {
	ticker := time.NewTicker(p.processStatusUpdatesInterval)
	p.waitGroup.Add(1)

	go func() {
		defer p.waitGroup.Done()

		statusUpdatesMap := map[chainhash.Hash]store.UpdateStatus{}

		for {
			select {
			case <-p.ctx.Done():
				return
			case statusUpdate := <-p.storageStatusUpdateCh:
				// Ensure no duplicate statuses
				updateStatusMap(statusUpdatesMap, statusUpdate)

				if len(statusUpdatesMap) >= p.processStatusUpdatesBatchSize {
					p.checkAndUpdate(statusUpdatesMap)
					statusUpdatesMap = map[chainhash.Hash]store.UpdateStatus{}

					// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
					// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
					ticker.Reset(p.processStatusUpdatesInterval)
				}
			case <-ticker.C:
				if len(statusUpdatesMap) > 0 {
					p.checkAndUpdate(statusUpdatesMap)
					statusUpdatesMap = map[chainhash.Hash]store.UpdateStatus{}

					// Reset ticker to delay the next tick, ensuring the interval starts after the batch is processed.
					// This prevents unnecessary immediate updates and maintains the intended time interval between batches.
					ticker.Reset(p.processStatusUpdatesInterval)
				}
			}
		}
	}()
}

func (p *Processor) checkAndUpdate(statusUpdatesMap map[chainhash.Hash]store.UpdateStatus) {
	if len(statusUpdatesMap) == 0 {
		return
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

	err := p.statusUpdateWithCallback(statusUpdates, doubleSpendUpdates)
	if err != nil {
		p.logger.Error("failed to bulk update statuses", slog.String("err", err.Error()))
	}
}

func (p *Processor) statusUpdateWithCallback(statusUpdates, doubleSpendUpdates []store.UpdateStatus) error {
	var updatedData []*store.StoreData
	var err error

	if len(statusUpdates) > 0 {
		updatedData, err = p.store.UpdateStatusBulk(context.Background(), statusUpdates)
		if err != nil {
			return err
		}
	}

	if len(doubleSpendUpdates) > 0 {
		updatedDoubleSpendData, err := p.store.UpdateDoubleSpend(context.Background(), doubleSpendUpdates)
		if err != nil {
			return err
		}
		updatedData = append(updatedData, updatedDoubleSpendData...)
	}

	for _, data := range updatedData {
		sendCallback := false
		if data.FullStatusUpdates {
			sendCallback = data.Status >= metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL
		} else {
			sendCallback = data.Status >= metamorph_api.Status_REJECTED
		}

		if sendCallback && len(data.Callbacks) > 0 {
			go p.callbackSender.SendCallback(p.logger, data)
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
				// Periodically read SEEN_ON_NETWORK transactions from database check their status in blocktx
				getSeenOnNetworkSince := p.now().Add(-1 * p.seenOnNetworkTxTime)
				getSeenOnNetworkUntil := p.now().Add(-1 * p.seenOnNetworkTxTimeUntil)
				var offset int64
				var totalSeenOnNetworkTxs int

				for {
					seenOnNetworkTxs, err := p.store.GetSeenOnNetwork(p.ctx, getSeenOnNetworkSince, getSeenOnNetworkUntil, loadSeenOnNetworkLimit, offset)
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
						if err = p.mqClient.Publish(RequestTxTopic, tx.Hash[:]); err != nil {
							p.logger.Error("failed to request tx from blocktx", slog.String("hash", tx.Hash.String()))
						}
					}
				}

				if totalSeenOnNetworkTxs > 0 {
					p.logger.Info("SEEN_ON_NETWORK txs being requested", slog.Int("number", totalSeenOnNetworkTxs))
				}
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
				// define from what point in time we are interested in unmined transactions
				getUnminedSince := p.now().Add(-1 * p.mapExpiryTime)
				var offset int64

				requested := 0
				announced := 0
				for {
					// get all transactions since then chunk by chunk
					unminedTxs, err := p.store.GetUnmined(p.ctx, getUnminedSince, loadUnminedLimit, offset)
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
						if err = p.store.IncrementRetries(p.ctx, tx.Hash); err != nil {
							p.logger.Error("Failed to increment retries in database", slog.String("err", err.Error()))
						}

						// every second time request tx, every other time announce tx
						if tx.Retries%2 == 0 {
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
			}
		}
	}()
}

// GetPeers returns a list of connected and a list of disconnected peers
func (p *Processor) GetPeers() []p2p.PeerI {
	return p.pm.GetPeers()
}

func (p *Processor) ProcessTransaction(ctx context.Context, req *ProcessorRequest) {
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
	if err = p.storeData(p.ctx, req.Data); err != nil {
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
	if err = p.mqClient.Publish(RegisterTxTopic, req.Data.Hash[:]); err != nil {
		p.logger.Error("failed to register tx in blocktx", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
	}

	// broadcast that transaction is stored to client
	statusResponse.UpdateStatus(StatusAndError{
		Status: metamorph_api.Status_STORED,
	})

	// Announce transaction to network peers
	p.logger.Debug("announcing transaction", slog.String("hash", req.Data.Hash.String()))
	peers := p.pm.AnnounceTransaction(req.Data.Hash, nil)
	if len(peers) == 0 {
		p.logger.Warn("transaction was not announced to any peer", slog.String("hash", req.Data.Hash.String()))
		return
	}

	// will be requesting transaction after ~2 seconds to get SEEN_ON_NETWORK status
	p.RequestTransaction(req.Data.Hash)

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
	p.responseProcessor.Add(statusResponse)
}

func (p *Processor) RequestTransaction(txHash *chainhash.Hash) {
	p.announcedTransactionsLock.Lock()
	p.announcedTransactions = append(p.announcedTransactions, AnnouncedTransaction{
		second: uint64(time.Now().Unix()),
		hash:   txHash,
	})
	p.announcedTransactionsLock.Unlock()
}

func (p *Processor) ProcessTransactions(sReq []*store.StoreData) {
	// store in database
	err := p.store.SetBulk(p.ctx, sReq)
	if err != nil {
		p.logger.Error("Failed to bulk store txs", slog.Int("number", len(sReq)), slog.String("err", err.Error()))
		return
	}

	for _, data := range sReq {
		// register transaction in blocktx using message queue
		err = p.mqClient.Publish(RegisterTxTopic, data.Hash[:])
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

var ErrUnhealthy = fmt.Errorf("processor has less than %d healthy peer connections", minimumHealthyConnectionsDefault)

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

func (p *Processor) storeData(ctx context.Context, data *store.StoreData) error {
	data.LastSubmittedAt = p.now()
	return p.store.Set(ctx, data)
}

func addNewCallback(data, reqData *store.StoreData) {
	if reqData.Callbacks == nil {
		return
	}
	reqCallback := reqData.Callbacks[0]
	if reqCallback.CallbackURL != "" && !callbackExists(reqCallback, data) {
		data.Callbacks = append(data.Callbacks, reqCallback)
	}
}

func callbackExists(callback store.StoreCallback, data *store.StoreData) bool {
	for _, c := range data.Callbacks {
		if c == callback {
			return true
		}
	}
	return false
}
