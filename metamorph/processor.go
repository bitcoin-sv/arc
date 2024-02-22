package metamorph

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

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
	// MaxRetries number of times we will retry announcing transaction if we haven't seen it on the network
	MaxRetries = 15
	// length of interval for checking transactions if they are seen on the network
	// if not we resend them again for a few times
	unseenTransactionRebroadcastingInterval = 60

	mapExpiryTimeDefault = 24 * time.Hour
	LogLevelDefault      = slog.LevelInfo

	failedToUpdateStatus       = "Failed to update status"
	dataRetentionPeriodDefault = 14 * 24 * time.Hour // 14 days

	maxMonitoriedTxs          = 100000
	loadUnminedLimit          = int64(5000)
	minimumHealthyConnections = 2

	processStatusUpdatesIntervalDefault  = 5 * time.Second
	processStatusUpdatesBatchSizeDefault = 100
)

var (
	ErrUnhealthy = errors.New("processor has less than 2 healthy peer connections")
)

type Processor struct {
	store                   store.MetamorphStore
	ProcessorResponseMap    *ProcessorResponseMap
	pm                      p2p.PeerManagerI
	mqClient                MessageQueueClient
	logger                  *slog.Logger
	mapExpiryTime           time.Duration
	dataRetentionPeriod     time.Duration
	now                     func() time.Time
	processExpiredTxsTicker *time.Ticker
	httpClient              HttpClient
	maxMonitoredTxs         int64

	quitListenTxChannel         chan struct{}
	quitListenTxChannelComplete chan struct{}
	minedTxsChan                chan *blocktx_api.TransactionBlocks

	statusUpdateCh                   chan store.UpdateStatus
	quitListenStatusUpdateCh         chan struct{}
	quitListenStatusUpdateChComplete chan struct{}
	processStatusUpdatesInterval     time.Duration
	processStatusUpdatesBatchSize    int

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
		startTime:               time.Now().UTC(),
		store:                   s,
		pm:                      pm,
		dataRetentionPeriod:     dataRetentionPeriodDefault,
		mapExpiryTime:           mapExpiryTimeDefault,
		now:                     time.Now,
		processExpiredTxsTicker: time.NewTicker(unseenTransactionRebroadcastingInterval * time.Second),
		maxMonitoredTxs:         maxMonitoriedTxs,

		quitListenTxChannel:         make(chan struct{}),
		quitListenTxChannelComplete: make(chan struct{}),

		quitListenStatusUpdateCh:         make(chan struct{}),
		quitListenStatusUpdateChComplete: make(chan struct{}),
		processStatusUpdatesInterval:     processStatusUpdatesIntervalDefault,
		processStatusUpdatesBatchSize:    processStatusUpdatesBatchSizeDefault,
		statusUpdateCh:                   make(chan store.UpdateStatus, processStatusUpdatesBatchSizeDefault),
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

	// Start a goroutine to resend transactions that have not been seen on the network
	go p.processExpiredTransactions()
	go p.processMinedCallbacks()
	go p.processStatusUpdates()

	gocore.AddAppPayloadFn("mtm", func() interface{} {
		return p.GetStats(false)
	})

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
	p.processExpiredTxsTicker.Stop()
	p.ProcessorResponseMap.Close()
	p.quitListenTxChannel <- struct{}{}
	<-p.quitListenTxChannelComplete
	p.quitListenStatusUpdateCh <- struct{}{}
	<-p.quitListenStatusUpdateChComplete
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

func (p *Processor) processMinedCallbacks() {
	go func() {
		defer func() {
			p.quitListenTxChannelComplete <- struct{}{}
		}()

		for {
			select {
			case <-p.quitListenTxChannel:
				return
			case txBlocks := <-p.minedTxsChan:
				updatedData, err := p.store.UpdateMined(context.Background(), txBlocks)
				if err != nil {
					p.logger.Error("failed to register transactions", slog.String("err", err.Error()))
				}

				for _, data := range updatedData {
					processorResponse, ok := p.ProcessorResponseMap.Get(data.Hash)
					if ok {
						p.mined.AddDuration(time.Since(processorResponse.Start))
						processorResponse.Close()
						p.ProcessorResponseMap.Delete(data.Hash)
					}

					if data.CallbackUrl != "" {
						go p.SendCallback(p.logger, data)
					}
				}
			}
		}
	}()
}

func (p *Processor) processStatusUpdates() {
	ticker := time.NewTicker(p.processStatusUpdatesInterval)

	go func() {
		defer func() {
			p.quitListenStatusUpdateChComplete <- struct{}{}
		}()

		statusUpdates := make([]store.UpdateStatus, 0, p.processStatusUpdatesBatchSize)
		statusUpdatesMap := map[chainhash.Hash]store.UpdateStatus{}

		for {
			select {
			case <-p.quitListenStatusUpdateCh:
				return
			case statusUpdate := <-p.statusUpdateCh:

				// Ensure no duplicate hashes, overwrite value if the status has higher value than existing status
				foundStatusUpdate, found := statusUpdatesMap[statusUpdate.Hash]
				if !found || (found && statusValueMap[foundStatusUpdate.Status] < statusValueMap[statusUpdate.Status]) {
					statusUpdatesMap[statusUpdate.Hash] = statusUpdate
				}

				if len(statusUpdatesMap) < p.processStatusUpdatesBatchSize {
					continue
				}

				for _, distinctStatusUpdate := range statusUpdatesMap {
					statusUpdates = append(statusUpdates, distinctStatusUpdate)
				}

				err := p.statusUpdateWithCallback(statusUpdates)
				if err != nil {
					p.logger.Error("failed to bulk update statuses", slog.String("err", err.Error()))
				}

				statusUpdates = make([]store.UpdateStatus, 0, p.processStatusUpdatesBatchSize)
				statusUpdatesMap = map[chainhash.Hash]store.UpdateStatus{}

			case <-ticker.C:
				if len(statusUpdatesMap) == 0 {
					continue
				}

				for _, distinctStatusUpdate := range statusUpdatesMap {
					statusUpdates = append(statusUpdates, distinctStatusUpdate)
				}

				err := p.statusUpdateWithCallback(statusUpdates)
				if err != nil {
					p.logger.Error("failed to bulk update statuses", slog.String("err", err.Error()))
				}

				statusUpdates = make([]store.UpdateStatus, 0, p.processStatusUpdatesBatchSize)
				statusUpdatesMap = map[chainhash.Hash]store.UpdateStatus{}

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
		if data.Status == metamorph_api.Status_SEEN_ON_NETWORK {
			processorResponse, ok := p.ProcessorResponseMap.Get(data.Hash)
			if ok {
				p.mined.AddDuration(time.Since(processorResponse.Start))
				processorResponse.Close()
				p.ProcessorResponseMap.Delete(data.Hash)
			}
		}

		if ((data.Status == metamorph_api.Status_SEEN_ON_NETWORK || data.Status == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL) && data.FullStatusUpdates || data.Status == metamorph_api.Status_REJECTED) && data.CallbackUrl != "" {
			go p.SendCallback(p.logger, data)
		}
	}
	return nil
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
				} else {
					p.logger.Debug("Re-announcing expired tx", slog.String("hash", txID.String()))
					p.pm.AnnounceTransaction(item.Hash, item.AnnouncedPeers)
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

func (p *Processor) LoadUnmined() {
	span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:LoadUnmined")
	defer span.Finish()

	limit := loadUnminedLimit
	margin := p.maxMonitoredTxs - int64(len(p.ProcessorResponseMap.Items()))

	if margin < limit {
		limit = margin
	}

	if limit <= 0 {
		return
	}

	p.logger.Info("loading unmined transactions", slog.Int64("limit", limit))
	getUnminedSince := p.now().Add(-1 * p.mapExpiryTime)

	unminedTxs, err := p.store.GetUnmined(spanCtx, getUnminedSince, limit)
	if err != nil {
		p.logger.Error("Failed to get unmined transactions", slog.String("err", err.Error()))
		return
	}

	if len(unminedTxs) == 0 {
		return
	}

	for _, record := range unminedTxs {
		pr := processor_response.NewProcessorResponseWithStatus(record.Hash, record.Status)
		pr.NoStats = true
		pr.Start = record.StoredAt
		p.ProcessorResponseMap.Set(record.Hash, pr)
	}
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
	processorResponse, ok := p.ProcessorResponseMap.Get(hash)
	if !ok {
		return nil
	}

	// Do not overwrite a higher value status with a lower or equal value status
	if statusValueMap[status] <= statusValueMap[processorResponse.GetStatus()] {
		p.logger.Debug("Status not updated for tx", slog.String("status", status.String()), slog.String("previous status", processorResponse.GetStatus().String()), slog.String("hash", hash.String()))

		return nil
	}

	span, _ := opentracing.StartSpanFromContext(context.Background(), "Processor:SendStatusForTransaction")
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

			p.statusUpdateCh <- store.UpdateStatus{
				Hash:         *hash,
				Status:       status,
				RejectReason: rejectReason,
			}

			return nil
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

			case metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL:
				p.seenInOrphanMempool.AddDuration(source, time.Since(processorResponse.Start))

			case metamorph_api.Status_SEEN_ON_NETWORK:
				p.seenOnNetwork.AddDuration(source, time.Since(processorResponse.Start))

			case metamorph_api.Status_REJECTED:
				p.rejected.AddDuration(source, time.Since(processorResponse.Start))
				p.logger.Warn("transaction rejected", slog.String("status", status.String()), slog.String("hash", hash.String()))
			}
		},
	}

	processorResponse.UpdateStatus(statusUpdate)

	return nil
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

	// check if tx already stored, return it
	data, err := p.store.Get(ctx, req.Data.Hash[:])
	if err == nil {
		var rejectErr error
		if data.RejectReason != "" {
			rejectErr = errors.New(data.RejectReason)
		}

		req.ResponseChannel <- processor_response.StatusAndError{
			Hash:   data.Hash,
			Status: data.Status,
			Err:    rejectErr,
		}

		return
	}

	go func() {
		err = p.mqClient.PublishRegisterTxs(req.Data.Hash[:])

		if err != nil {
			p.logger.Error("failed to register tx in blocktx", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
		}
	}()

	processorResponse := processor_response.NewProcessorResponseWithChannel(req.Data.Hash, req.ResponseChannel)

	// STEP 1: STORED
	processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
		Status: metamorph_api.Status_STORED,
		Source: "processor",
		UpdateStore: func() error {
			req.Data.Status = metamorph_api.Status_STORED
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

			p.logger.Info("announcing transaction", slog.String("hash", req.Data.Hash.String()))
			// STEP 2: ANNOUNCED_TO_NETWORK
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

					p.statusUpdateCh <- store.UpdateStatus{
						Hash:         *req.Data.Hash,
						Status:       metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						RejectReason: "",
					}

					return nil
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
}

func (p *Processor) Health() error {
	healthyConnections := 0

	for _, peer := range p.pm.GetPeers() {
		if peer.Connected() && peer.IsHealthy() {
			healthyConnections++
		}
	}

	if healthyConnections < minimumHealthyConnections {
		return ErrUnhealthy
	}

	return nil
}
