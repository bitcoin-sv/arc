package metamorph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
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
)

type Processor struct {
	store                     store.MetamorphStore
	hostname                  string
	ProcessorResponseMap      *ProcessorResponseMap
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

	statusMessageCh         chan *PeerTxMessage
	CancelSendStatusMessage context.CancelFunc

	waitGroup *sync.WaitGroup

	statCollectionInterval time.Duration

	cancelAll context.CancelFunc
	ctx       context.Context

	lockTransactionsInterval time.Duration

	minedTxsChan chan *blocktx_api.TransactionBlocks

	storageStatusUpdateCh         chan store.UpdateStatus
	processStatusUpdatesInterval  time.Duration
	processStatusUpdatesBatchSize int

	processExpiredTxsInterval       time.Duration
	processSeenOnNetworkTxsInterval time.Duration
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
		statusMessageCh:           statusMessageChannel,

		processExpiredTxsInterval:       unseenTransactionRebroadcastingInterval,
		processSeenOnNetworkTxsInterval: seenOnNetworkTransactionRequestingInterval,
		lockTransactionsInterval:        unseenTransactionRebroadcastingInterval,

		processStatusUpdatesInterval:  processStatusUpdatesIntervalDefault,
		processStatusUpdatesBatchSize: processStatusUpdatesBatchSizeDefault,
		storageStatusUpdateCh:         make(chan store.UpdateStatus, processStatusUpdatesBatchSizeDefault),
		stats:                         newProcessorStats(),
		waitGroup:                     &sync.WaitGroup{},

		statCollectionInterval: statCollectionIntervalDefault,
	}

	p.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault})).With(slog.String("service", "mtm"))

	// apply options to processor
	for _, opt := range opts {
		opt(p)
	}

	p.ProcessorResponseMap = NewProcessorResponseMap(p.mapExpiryTime, WithNowResponseMap(p.now))

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

	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case txBlocks := <-p.minedTxsChan:
				if txBlocks == nil {
					continue
				}

				updatedData, err := p.store.UpdateMined(p.ctx, txBlocks)
				if err != nil {
					p.logger.Error("failed to register transactions", slog.String("err", err.Error()))
					return
				}

				for _, data := range updatedData {
					if data.CallbackUrl != "" {
						go p.callbackSender.SendCallback(p.logger, data)
					}
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

			case message := <-p.statusMessageCh:
				p.SendStatusForTransaction(message.Hash, message.Status, message.Err)
			}
		}
	}()
}

func (p *Processor) CheckAndUpdate(statusUpdatesMap map[chainhash.Hash]store.UpdateStatus) {
	if len(statusUpdatesMap) == 0 {
		return
	}

	statusUpdates := make([]store.UpdateStatus, 0, p.processStatusUpdatesBatchSize)
	for _, distinctStatusUpdate := range statusUpdatesMap {
		statusUpdates = append(statusUpdates, distinctStatusUpdate)
	}

	err := p.statusUpdateWithCallback(statusUpdates)
	if err != nil {
		p.logger.Error("failed to bulk update statuses", slog.String("err", err.Error()))
	}
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
				// Ensure no duplicate hashes, overwrite value if the status has higher value than existing status
				foundStatusUpdate, found := statusUpdatesMap[statusUpdate.Hash]
				if !found || (found && statusValueMap[foundStatusUpdate.Status] < statusValueMap[statusUpdate.Status]) {
					statusUpdatesMap[statusUpdate.Hash] = statusUpdate
				}

				if len(statusUpdatesMap) >= p.processStatusUpdatesBatchSize {
					p.CheckAndUpdate(statusUpdatesMap)
					statusUpdatesMap = map[chainhash.Hash]store.UpdateStatus{}
				}
			case <-ticker.C:
				if len(statusUpdatesMap) > 0 {
					p.CheckAndUpdate(statusUpdatesMap)
					statusUpdatesMap = map[chainhash.Hash]store.UpdateStatus{}
				}
			}
		}
	}()
}

func (p *Processor) statusUpdateWithCallback(statusUpdates []store.UpdateStatus) error {
	updatedData, err := p.store.UpdateStatusBulk(context.Background(), statusUpdates)
	if err != nil {
		return err
	}

	for _, data := range updatedData {
		if ((data.Status == metamorph_api.Status_SEEN_ON_NETWORK || data.Status == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL) && data.FullStatusUpdates || data.Status == metamorph_api.Status_REJECTED) && data.CallbackUrl != "" {
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
						continue
					}

					if len(seenOnNetworkTxs) == 0 {
						break
					}

					totalSeenOnNetworkTxs += len(seenOnNetworkTxs)

					for _, tx := range seenOnNetworkTxs {
						// by requesting tx, blocktx checks if it has the transaction mined in the database and sends it back
						if err = p.mqClient.PublishRequestTx(tx.Hash[:]); err != nil {
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
							// Sending GETDATA to peers to see if they have it
							p.logger.Debug("Re-getting expired tx", slog.String("hash", tx.Hash.String()))
							p.pm.RequestTransaction(tx.Hash)
							requested++
							continue
						}

						p.logger.Debug("Re-announcing expired tx", slog.String("hash", tx.Hash.String()))
						peers := p.pm.AnnounceTransaction(tx.Hash, nil)
						if len(peers) == 0 {
							p.logger.Warn("transaction was not announced to any peer during rebroadcast", slog.String("hash", tx.Hash.String()))
						}
						announced++
					}
				}

				if announced > 0 || requested > 0 {
					p.logger.Info("Retried unmined transactions", slog.Int("announced", announced), slog.Int("requested", requested), slog.String("since", getUnminedSince.String()))
				}
			}
		}
	}()
}

// GetPeers returns a list of connected and a list of disconnected peers
func (p *Processor) GetPeers() []p2p.PeerI {
	return p.pm.GetPeers()
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

func (p *Processor) SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, statusErr error) {
	// make sure we update the transaction status in database
	var rejectReason string
	if statusErr != nil {
		rejectReason = statusErr.Error()
	}

	p.storageStatusUpdateCh <- store.UpdateStatus{
		Hash:         *hash,
		Status:       status,
		RejectReason: rejectReason,
	}

	// if we receive new update check if we have client connection waiting for status and send it
	processorResponse, ok := p.ProcessorResponseMap.Get(hash)
	if ok {
		processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
			Status:    status,
			StatusErr: statusErr,
		})
	}

	p.logger.Debug("Status reported for tx", slog.String("status", status.String()), slog.String("hash", hash.String()))
}

func (p *Processor) ProcessTransaction(ctx context.Context, req *ProcessorRequest) {
	// we need to decouple the Context from the request, so that we don't get cancelled
	// when the request is cancelled

	// check if tx already stored, return it
	data, err := p.store.Get(ctx, req.Data.Hash[:])
	if err == nil {
		/*
			When transaction is re-submitted we make last_submitted_at to be now()
			to make sure it will be loaded and re-broadcasted if needed.
		*/
		_ = p.storeData(ctx, data)

		var rejectErr error
		if data.RejectReason != "" {
			rejectErr = errors.New(data.RejectReason)
		}

		// notify the client instantly and return without waiting for any specific status
		req.ResponseChannel <- processor_response.StatusAndError{
			Hash:   data.Hash,
			Status: data.Status,
			Err:    rejectErr,
		}

		return
	}

	if !errors.Is(err, store.ErrNotFound) {
		// issue with the store itself
		// notify the client instantly and return
		req.ResponseChannel <- processor_response.StatusAndError{
			Hash:   req.Data.Hash,
			Status: metamorph_api.Status_RECEIVED,
			Err:    err,
		}

		return
	}

	// store in database
	req.Data.Status = metamorph_api.Status_STORED
	if err = p.storeData(ctx, req.Data); err != nil {
		// issue with the store itself
		// notify the client instantly and return
		req.ResponseChannel <- processor_response.StatusAndError{
			Hash:   req.Data.Hash,
			Status: metamorph_api.Status_RECEIVED,
			Err:    err,
		}
		return
	}

	// register transaction in blocktx using message queue
	if err = p.mqClient.PublishRegisterTxs(req.Data.Hash[:]); err != nil {
		p.logger.Error("failed to register tx in blocktx", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
	}

	processorResponse := processor_response.NewProcessorResponseWithChannel(req.Data.Hash, req.ResponseChannel)

	// broadcast that transaction is stored to client
	processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
		Status:    metamorph_api.Status_STORED,
		StatusErr: err,
	})

	// Add this transaction to the map of transactions that client is listening to with open connection.
	p.ProcessorResponseMap.Set(req.Data.Hash, processorResponse)

	// we no longer need processor response object after client disconnects
	go func() {
		time.Sleep(req.Timeout + time.Second)
		p.ProcessorResponseMap.Delete(req.Data.Hash)
	}()

	// Announce transaction to network and save peers
	p.logger.Debug("announcing transaction", slog.String("hash", req.Data.Hash.String()))
	peers := p.pm.AnnounceTransaction(req.Data.Hash, nil)
	if len(peers) == 0 {
		p.logger.Warn("transaction was not announced to any peer", slog.String("hash", req.Data.Hash.String()))
		return
	}

	// notify existing client about new status
	processorResponse, ok := p.ProcessorResponseMap.Get(req.Data.Hash)
	if ok {
		processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
			Status:    metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			StatusErr: nil,
		})
	}

	// broadcast that transaction is announced to network (eventually active clientს catch that)
	p.storageStatusUpdateCh <- store.UpdateStatus{
		Hash:         *req.Data.Hash,
		Status:       metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		RejectReason: "",
	}
}

var (
	ErrUnhealthy = fmt.Errorf("processor has less than %d healthy peer connections", minimumHealthyConnectionsDefault)
)

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
	/*
		We make last_submitted_at to be now()
		to make sure it will be loaded and (re-)broadcasted if needed.
	*/

	data.LastSubmittedAt = p.now()

	err := p.store.Set(ctx, data.Hash[:], data)
	if err != nil {
		p.logger.Error("Failed to store transaction", slog.String("hash", data.Hash.String()), slog.String("err", err.Error()))
	}

	return err
}
