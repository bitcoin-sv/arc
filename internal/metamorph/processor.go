package metamorph

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-utils/stat"
)

const (
	// MaxRetries number of times we will retry announcing transaction if we haven't seen it on the network
	MaxRetries = 15
	// length of interval for checking transactions if they are seen on the network
	// if not we resend them again for a few times
	unseenTransactionRebroadcastingInterval = 60 * time.Second

	mapExpiryTimeDefault = 24 * time.Hour
	LogLevelDefault      = slog.LevelInfo

	loadUnminedLimit          = int64(5000)
	minimumHealthyConnections = 2

	processStatusUpdatesIntervalDefault  = 500 * time.Millisecond
	processStatusUpdatesBatchSizeDefault = 1000
)

var (
	ErrUnhealthy = errors.New("processor has less than 2 healthy peer connections")
)

type Processor struct {
	store                store.MetamorphStore
	ProcessorResponseMap *ProcessorResponseMap
	pm                   p2p.PeerManagerI
	mqClient             MessageQueueClient
	logger               *slog.Logger
	mapExpiryTime        time.Duration
	now                  func() time.Time

	httpClient HttpClient

	lockTransactionsInterval     time.Duration
	quitLockTransactions         chan struct{}
	quitLockTransactionsComplete chan struct{}

	quitProcessMinedCallbacks         chan struct{}
	quitProcessMinedCallbacksComplete chan struct{}
	minedTxsChan                      chan *blocktx_api.TransactionBlocks

	storageStatusUpdateCh                     chan store.UpdateStatus
	quitProcessStatusUpdatesInStorage         chan struct{}
	quitProcessStatusUpdatesInStorageComplete chan struct{}
	processStatusUpdatesInterval              time.Duration
	processStatusUpdatesBatchSize             int

	processExpiredTxsInterval              time.Duration
	quitProcessExpiredTransactions         chan struct{}
	quitProcessExpiredTransactionsComplete chan struct{}

	startTime           time.Time
	queueLength         atomic.Int32
	queuedCount         atomic.Int32
	stored              *stat.AtomicStat
	announcedToNetwork  *stat.AtomicStats
	requestedByNetwork  *stat.AtomicStats
	sentToNetwork       *stat.AtomicStats
	acceptedByNetwork   *stat.AtomicStats
	seenInOrphanMempool *stat.AtomicStats
	seenOnNetwork       *stat.AtomicStats
	rejected            *stat.AtomicStats
	mined               *stat.AtomicStat
	retries             *stat.AtomicStat
}

type Option func(f *Processor)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewProcessor(s store.MetamorphStore, pm p2p.PeerManagerI, opts ...Option) (*Processor, error) {
	if s == nil {
		return nil, errors.New("store cannot be nil")
	}

	if pm == nil {
		return nil, errors.New("peer manager cannot be nil")
	}

	p := &Processor{
		startTime:     time.Now().UTC(),
		store:         s,
		pm:            pm,
		mapExpiryTime: mapExpiryTimeDefault,
		now:           time.Now,

		processExpiredTxsInterval: unseenTransactionRebroadcastingInterval,
		lockTransactionsInterval:  unseenTransactionRebroadcastingInterval,

		processStatusUpdatesInterval:  processStatusUpdatesIntervalDefault,
		processStatusUpdatesBatchSize: processStatusUpdatesBatchSizeDefault,
		storageStatusUpdateCh:         make(chan store.UpdateStatus, processStatusUpdatesBatchSizeDefault),

		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		stored:              stat.NewAtomicStat(),
		announcedToNetwork:  stat.NewAtomicStats(),
		requestedByNetwork:  stat.NewAtomicStats(),
		sentToNetwork:       stat.NewAtomicStats(),
		acceptedByNetwork:   stat.NewAtomicStats(),
		seenInOrphanMempool: stat.NewAtomicStats(),
		seenOnNetwork:       stat.NewAtomicStats(),
		rejected:            stat.NewAtomicStats(),
		mined:               stat.NewAtomicStat(),
		retries:             stat.NewAtomicStat(),
	}

	p.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault})).With(slog.String("service", "mtm"))

	// apply options to processor
	for _, opt := range opts {
		opt(p)
	}

	p.ProcessorResponseMap = NewProcessorResponseMap(p.mapExpiryTime, WithNowResponseMap(p.now))

	p.logger.Info("Starting processor", slog.String("cacheExpiryTime", p.mapExpiryTime.String()))

	_ = newPrometheusCollector(p)

	return p, nil
}

// Shutdown closes all channels and goroutines gracefully
func (p *Processor) Shutdown() {
	p.logger.Info("Shutting down processor")

	err := p.unlockItems()
	if err != nil {
		p.logger.Error("Failed to unlock all hashes", slog.String("err", err.Error()))
	}

	if p.quitLockTransactions != nil {
		p.quitLockTransactions <- struct{}{}
		<-p.quitLockTransactionsComplete
	}
	if p.quitProcessMinedCallbacks != nil {
		p.quitProcessMinedCallbacks <- struct{}{}
		<-p.quitProcessMinedCallbacksComplete
	}

	if p.quitProcessStatusUpdatesInStorage != nil {
		p.quitProcessStatusUpdatesInStorage <- struct{}{}
		<-p.quitProcessStatusUpdatesInStorageComplete
	}

	if p.quitProcessExpiredTransactions != nil {
		p.quitProcessExpiredTransactions <- struct{}{}
		<-p.quitProcessExpiredTransactionsComplete
	}
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

	if len(hashes) > 0 {
		p.logger.Info("unlocking items", slog.Int("number", len(hashes)))
		return p.store.SetUnlocked(context.Background(), hashes)
	}

	return nil
}

func (p *Processor) StartProcessMinedCallbacks() {

	p.quitProcessMinedCallbacks = make(chan struct{})
	p.quitProcessMinedCallbacksComplete = make(chan struct{})

	go func() {
		defer func() {
			p.quitProcessMinedCallbacksComplete <- struct{}{}
		}()

		for {
			select {
			case <-p.quitProcessMinedCallbacks:
				return
			case txBlocks := <-p.minedTxsChan:
				updatedData, err := p.store.UpdateMined(context.Background(), txBlocks)
				if err != nil {
					p.logger.Error("failed to register transactions", slog.String("err", err.Error()))
				}

				for _, data := range updatedData {
					if data.CallbackUrl != "" {
						go p.SendCallback(p.logger, data)
					}
				}
			}
		}
	}()
}

func (p *Processor) CheckAndUpdate(statusUpdatesMap *map[chainhash.Hash]store.UpdateStatus) {
	if len(*statusUpdatesMap) == 0 {
		return
	}

	statusUpdates := make([]store.UpdateStatus, 0, p.processStatusUpdatesBatchSize)
	for _, distinctStatusUpdate := range *statusUpdatesMap {
		statusUpdates = append(statusUpdates, distinctStatusUpdate)
	}

	err := p.statusUpdateWithCallback(statusUpdates)
	if err != nil {
		p.logger.Error("failed to bulk update statuses", slog.String("err", err.Error()))
	}

	*statusUpdatesMap = map[chainhash.Hash]store.UpdateStatus{}
}

func (p *Processor) StartProcessStatusUpdatesInStorage() {
	ticker := time.NewTicker(p.processStatusUpdatesInterval)

	p.quitProcessStatusUpdatesInStorage = make(chan struct{})
	p.quitProcessStatusUpdatesInStorageComplete = make(chan struct{})

	go func() {
		defer func() {
			p.quitProcessStatusUpdatesInStorageComplete <- struct{}{}
		}()

		statusUpdatesMap := map[chainhash.Hash]store.UpdateStatus{}

		for {
			select {
			case <-p.quitProcessStatusUpdatesInStorage:
				return
			case statusUpdate := <-p.storageStatusUpdateCh:
				// Ensure no duplicate hashes, overwrite value if the status has higher value than existing status
				foundStatusUpdate, found := statusUpdatesMap[statusUpdate.Hash]
				if !found || (found && statusValueMap[foundStatusUpdate.Status] < statusValueMap[statusUpdate.Status]) {
					statusUpdatesMap[statusUpdate.Hash] = statusUpdate
				}

				if len(statusUpdatesMap) >= p.processStatusUpdatesBatchSize {
					p.CheckAndUpdate(&statusUpdatesMap)
				}
			case <-ticker.C:
				p.CheckAndUpdate(&statusUpdatesMap)
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
			go p.SendCallback(p.logger, data)
		}
	}
	return nil
}

func (p *Processor) StartLockTransactions() {
	span, _ := opentracing.StartSpanFromContext(context.Background(), "Processor:lockTransactions")
	dbctx := opentracing.ContextWithSpan(context.Background(), span)
	defer span.Finish()
	ticker := time.NewTicker(p.lockTransactionsInterval)

	p.quitLockTransactions = make(chan struct{})
	p.quitLockTransactionsComplete = make(chan struct{})

	go func() {
		defer func() {
			p.quitLockTransactionsComplete <- struct{}{}
		}()
		for {
			select {
			case <-p.quitLockTransactions:
				return
			case <-ticker.C:
				expiredSince := p.now().Add(-1 * p.mapExpiryTime)
				err := p.store.SetLocked(dbctx, expiredSince, loadUnminedLimit)
				if err != nil {
					p.logger.Error("Failed to set transactions locked", slog.String("err", err.Error()))
				}
			}
		}
	}()
}

func (p *Processor) StartProcessExpiredTransactions() {
	span, _ := opentracing.StartSpanFromContext(context.Background(), "Processor:processExpiredTransactions")
	dbctx := opentracing.ContextWithSpan(context.Background(), span)

	ticker := time.NewTicker(p.processExpiredTxsInterval)

	p.quitProcessExpiredTransactions = make(chan struct{})
	p.quitProcessExpiredTransactionsComplete = make(chan struct{})

	defer span.Finish()
	go func() {
		defer func() {
			p.quitProcessExpiredTransactionsComplete <- struct{}{}
		}()

		for {
			select {
			case <-p.quitProcessExpiredTransactions:
				return
			case <-ticker.C: // Periodically read unmined transactions from database and announce them again
				// define from what point in time we are interested in unmined transactions
				getUnminedSince := p.now().Add(-1 * p.mapExpiryTime)
				var offset int64

				for {
					// get all transactions since then chunk by chunk
					unminedTxs, err := p.store.GetUnmined(dbctx, getUnminedSince, loadUnminedLimit, offset)
					if err != nil {
						p.logger.Error("Failed to get unmined transactions", slog.String("err", err.Error()))
						continue
					}

					offset += loadUnminedLimit
					if len(unminedTxs) == 0 {
						break
					}

					requested := 0
					announced := 0
					for _, tx := range unminedTxs {
						// mark that we retried processing this transaction once more
						if err = p.store.IncrementRetries(dbctx, tx.Hash); err != nil {
							p.logger.Error("Failed to increment retries in database", slog.String("err", err.Error()))
						}

						if tx.Retries > MaxRetries {
							// Sending GETDATA to peers to see if they have it
							p.logger.Debug("Re-getting expired tx", slog.String("hash", tx.Hash.String()))
							p.pm.RequestTransaction(tx.Hash)
							requested++
						} else {
							p.logger.Debug("Re-announcing expired tx", slog.String("hash", tx.Hash.String()))
							p.pm.AnnounceTransaction(tx.Hash, nil)
							announced++
						}

						p.retries.AddDuration(time.Since(time.Now()))
					}

					p.logger.Info("Retried unmined transactions", slog.Int("announced", announced), slog.Int("requested", requested))
				}
			}
		}
	}()
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

func (p *Processor) SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, source string, statusErr error) error {
	span, _ := opentracing.StartSpanFromContext(context.Background(), "Processor:SendStatusForTransaction")
	defer span.Finish()

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
			StatusErr: nil,
		})
	}

	p.logger.Debug("Status reported for tx", slog.String("status", status.String()), slog.String("hash", hash.String()))
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

	// check if tx already stored, return it
	data, err := p.store.Get(ctx, req.Data.Hash[:])
	if err == nil {
		/*
			When transaction is re-submitted we make inserted_at_num to be now()
			to make sure it will be loaded and re-broadcasted if needed.
		*/
		insertedAtNum, _ := strconv.Atoi(p.now().Format("2006010215"))
		data.InsertedAtNum = insertedAtNum
		if setErr := p.store.Set(spanCtx, req.Data.Hash[:], data); setErr != nil {
			p.logger.Error("Failed to store transaction", slog.String("hash", req.Data.Hash.String()), slog.String("err", setErr.Error()))
		}

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

	// register transaction in blocktx using message queue
	if err = p.mqClient.PublishRegisterTxs(req.Data.Hash[:]); err != nil {
		p.logger.Error("failed to register tx in blocktx", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
	}

	processorResponse := processor_response.NewProcessorResponseWithChannel(req.Data.Hash, req.ResponseChannel, req.Timeout)

	// store in database
	req.Data.Status = metamorph_api.Status_STORED
	insertedAtNum, _ := strconv.Atoi(p.now().Format("2006010215"))
	req.Data.InsertedAtNum = insertedAtNum
	err = p.store.Set(spanCtx, req.Data.Hash[:], req.Data)
	if err != nil {
		p.logger.Error("Failed to store transaction", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
	}

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
	p.pm.AnnounceTransaction(req.Data.Hash, nil)

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

func (p *Processor) Health() error {
	healthyConnections := 0

	for _, peer := range p.pm.GetPeers() {
		if peer.Connected() && peer.IsHealthy() {
			healthyConnections++
		}
	}

	if healthyConnections < minimumHealthyConnections {
		p.logger.Warn("Less than expected healthy peers", slog.Int("number", healthyConnections))
		return nil
	}

	if healthyConnections == 0 {
		p.logger.Error("Metamorph not healthy")
		return ErrUnhealthy
	}

	return nil
}
