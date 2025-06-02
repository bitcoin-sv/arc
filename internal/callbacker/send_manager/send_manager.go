package send_manager

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type SendManagerStore interface {
	Set(ctx context.Context, dto *store.CallbackData) error
	SetMany(ctx context.Context, data []*store.CallbackData) error
	GetAndDelete(ctx context.Context, url string, limit int) ([]*store.CallbackData, error)
}

type Sender interface {
	Send(url, token string, callback *callbacker.Callback) (success, retry bool)
	SendBatch(url, token string, callbacks []*callbacker.Callback) (success, retry bool)
}

type SendManager struct {
	url string

	// dependencies
	sender Sender
	store  SendManagerStore
	logger *slog.Logger

	expiration time.Duration

	// internal state
	entriesWg sync.WaitGroup
	cancelAll context.CancelFunc
	ctx       context.Context

	queueProcessInterval    time.Duration
	fillUpQueueInterval     time.Duration
	sortByTimestampInterval time.Duration
	batchSendInterval       time.Duration
	batchSize               int

	bufferSize int
	now        func() time.Time

	mu            sync.Mutex
	callbackQueue []*callbacker.CallbackEntry
}

const (
	entriesBufferSize              = 10000
	batchSizeDefault               = 50
	queueProcessIntervalDefault    = 5 * time.Second
	fillUpQueueIntervalDefault     = 5 * time.Second
	expirationDefault              = 24 * time.Hour
	sortByTimestampIntervalDefault = 10 * time.Second
	batchSendIntervalDefault       = 5 * time.Second
)

var (
	ErrSendBatchedCallbacks = errors.New("failed to send batched callback")
)

func WithNow(nowFunc func() time.Time) func(*SendManager) {
	return func(m *SendManager) {
		m.now = nowFunc
	}
}

func WithBufferSize(size int) func(*SendManager) {
	return func(m *SendManager) {
		if size >= entriesBufferSize {
			m.bufferSize = entriesBufferSize
			return
		}
		m.bufferSize = size
	}
}

func WithQueueProcessInterval(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.queueProcessInterval = d
	}
}

func WithBackfillQueueInterval(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.fillUpQueueInterval = d
	}
}

func WithExpiration(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.expiration = d
	}
}

func WithSortByTimestampInterval(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.sortByTimestampInterval = d
	}
}

func WithBatchSendInterval(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.batchSendInterval = d
	}
}

func WithBatchSize(size int) func(*SendManager) {
	return func(m *SendManager) {
		m.batchSize = size
	}
}

func New(url string, sender callbacker.SenderI, store SendManagerStore, logger *slog.Logger, opts ...func(*SendManager)) *SendManager {
	m := &SendManager{
		url:    url,
		sender: sender,
		store:  store,
		logger: logger,

		queueProcessInterval:    queueProcessIntervalDefault,
		expiration:              expirationDefault,
		fillUpQueueInterval:     fillUpQueueIntervalDefault,
		sortByTimestampInterval: sortByTimestampIntervalDefault,
		batchSendInterval:       batchSendIntervalDefault,
		batchSize:               batchSizeDefault,

		callbackQueue: make([]*callbacker.CallbackEntry, 0, entriesBufferSize),
		bufferSize:    entriesBufferSize,

		now: time.Now,
	}

	for _, opt := range opts {
		opt(m)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	m.cancelAll = cancelAll
	m.ctx = ctx

	return m
}

func (m *SendManager) Enqueue(entry callbacker.CallbackEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.callbackQueue) >= m.bufferSize {
		m.storeToDB(entry)
		return
	}

	m.callbackQueue = append(m.callbackQueue, &entry)
}

func (m *SendManager) CallbacksQueued() int {
	return len(m.callbackQueue)
}

func (m *SendManager) Start() {
	queueTicker := time.NewTicker(m.queueProcessInterval)
	sortQueueTicker := time.NewTicker(m.sortByTimestampInterval)
	backfillQueueTicker := time.NewTicker(m.fillUpQueueInterval)
	batchSendTicker := time.NewTicker(m.batchSendInterval)

	m.entriesWg.Add(1)
	var callbackBatch []*callbacker.CallbackEntry

	go func() {
		var err error
		defer func() {
			m.storeRemainingCallbacks(callbackBatch)
		}()
		lastIterationWasBatch := false

		for {
			const queueLength = "queue length"
			const failedToSendCallbacks = "Failed to send batch of callbacks"
			const callbackElements = "callback elements"
			select {
			case <-m.ctx.Done():
				return
			case <-sortQueueTicker.C:
				m.mu.Lock()
				sort.Slice(m.callbackQueue, func(i, j int) bool {
					return m.callbackQueue[j].Data.Timestamp.After(m.callbackQueue[i].Data.Timestamp)
				})
				m.mu.Unlock()

			case <-backfillQueueTicker.C:
				m.fillUpQueue()
			case <-batchSendTicker.C:
				if len(callbackBatch) == 0 {
					continue
				}

				err = m.sendElementBatch(callbackBatch)
				if err != nil {
					m.logger.Warn(failedToSendCallbacks, slog.String("url", m.url))
					continue
				}

				callbackBatch = callbackBatch[:0]
				m.logger.Debug("Batched callbacks sent on interval", slog.Int(callbackElements, len(callbackBatch)), slog.Int(queueLength, m.CallbacksQueued()), slog.String("url", m.url))
			case <-queueTicker.C:
				if len(m.callbackQueue) == 0 {
					continue
				}
				callbackEntry := m.callbackQueue[0]
				if callbackEntry == nil {
					continue
				}

				// If item is expired - dequeue without storing
				if m.now().Sub(callbackEntry.Data.Timestamp) > m.expiration {
					m.logger.Warn("Callback expired", slog.Time("timestamp", callbackEntry.Data.Timestamp), slog.String("hash", callbackEntry.Data.TxID), slog.String("status", callbackEntry.Data.TxStatus))
					m.callbackQueue = m.callbackQueue[1:]
					continue
				}

				if callbackEntry.AllowBatch {
					lastIterationWasBatch = true

					if len(callbackBatch) < m.batchSize {
						callbackBatch = append(callbackBatch, callbackEntry)
						queueTicker.Reset(m.queueProcessInterval)
						m.callbackQueue = m.callbackQueue[1:]
						continue
					}

					err = m.sendElementBatch(callbackBatch)
					if err != nil {
						m.logger.Warn(failedToSendCallbacks, slog.String("url", m.url))
						continue
					}

					callbackBatch = callbackBatch[:0]
					m.logger.Debug("Batched callbacks sent", slog.Int(callbackElements, len(callbackBatch)), slog.Int(queueLength, m.CallbacksQueued()), slog.String("url", m.url))
					continue
				}

				if lastIterationWasBatch {
					lastIterationWasBatch = false
					if len(callbackBatch) > 0 {
						// if entry is not a batched entry, but last one was, send batch to keep the order
						err = m.sendElementBatch(callbackBatch)
						if err != nil {
							m.logger.Error(failedToSendCallbacks, slog.String("url", m.url))
							continue
						}
						callbackBatch = callbackBatch[:0]
						m.logger.Debug("Batched callbacks sent before sending single callback", slog.Int(callbackElements, len(callbackBatch)), slog.Int(queueLength, m.CallbacksQueued()), slog.String("url", m.url))
					}
				}

				success, retry := m.sender.Send(m.url, callbackEntry.Token, callbackEntry.Data)
				if !retry || success {
					m.callbackQueue = m.callbackQueue[1:]
					m.logger.Debug("Single callback sent", slog.Int(callbackElements, len(callbackBatch)), slog.Int(queueLength, m.CallbacksQueued()), slog.String("url", m.url))
					continue
				}
				m.logger.Warn("Failed to send single callback", slog.String("url", m.url))
			}
		}
	}()
}

func (m *SendManager) sendElementBatch(callbackElements []*callbacker.CallbackEntry) error {
	callbackBatch := make([]callbacker.CallbackEntry, 0, len(callbackElements))
	for _, callback := range callbackElements {
		callbackBatch = append(callbackBatch, *callback)
	}
	success, retry := m.sendBatch(callbackBatch)
	if !retry || success {
		m.mu.Lock()
		for _, callbackElement := range callbackElements {
			for i, cb := range m.callbackQueue {
				if cb == callbackElement {
					m.callbackQueue = append(m.callbackQueue[:i], m.callbackQueue[i+1:]...)
				}
			}
		}
		m.mu.Unlock()

		return nil
	}

	return ErrSendBatchedCallbacks
}

func (m *SendManager) sendBatch(batch []callbacker.CallbackEntry) (success, retry bool) {
	token := batch[0].Token
	callbacks := make([]*callbacker.Callback, len(batch))
	for i, e := range batch {
		callbacks[i] = e.Data
	}

	return m.sender.SendBatch(m.url, token, callbacks)
}

// fillUpQueue calculates the capacity left in the queue and fills it up
func (m *SendManager) fillUpQueue() {
	capacityLeft := m.bufferSize - len(m.callbackQueue)
	if capacityLeft == 0 {
		return
	}

	callbacks, err := m.store.GetAndDelete(m.ctx, m.url, capacityLeft)
	if err != nil {
		m.logger.Error("Failed to load callbacks", slog.String("err", err.Error()))
		return
	}

	for _, callback := range callbacks {
		m.Enqueue(toEntry(callback))
	}

	if len(callbacks) > 0 {
		m.logger.Debug("Callback queue filled up", slog.Int("callback elements", len(callbacks)), slog.Int("queue length", m.CallbacksQueued()), slog.String("url", m.url))
	}
}

func (m *SendManager) storeToDB(entry callbacker.CallbackEntry) {
	callbackData := toStoreDto(m.url, entry)
	err := m.store.Set(m.ctx, callbackData)
	if err != nil {
		m.logger.Error("Failed to set callback data", slog.String("hash", callbackData.TxID), slog.String("status", callbackData.TxStatus), slog.String("err", err.Error()))
	}
}

func toStoreDto(url string, entry callbacker.CallbackEntry) *store.CallbackData {
	return &store.CallbackData{
		URL:       url,
		Token:     entry.Token,
		Timestamp: entry.Data.Timestamp,

		CompetingTxs: entry.Data.CompetingTxs,
		TxID:         entry.Data.TxID,
		TxStatus:     entry.Data.TxStatus,
		ExtraInfo:    entry.Data.ExtraInfo,
		MerklePath:   entry.Data.MerklePath,

		BlockHash:   entry.Data.BlockHash,
		BlockHeight: entry.Data.BlockHeight,

		AllowBatch: entry.AllowBatch,
	}
}

func toEntry(callbackData *store.CallbackData) callbacker.CallbackEntry {
	return callbacker.CallbackEntry{
		Token: callbackData.Token,
		Data: &callbacker.Callback{
			Timestamp:    callbackData.Timestamp,
			CompetingTxs: callbackData.CompetingTxs,
			TxID:         callbackData.TxID,
			TxStatus:     callbackData.TxStatus,
			ExtraInfo:    callbackData.ExtraInfo,
			MerklePath:   callbackData.MerklePath,
			BlockHash:    callbackData.BlockHash,
			BlockHeight:  callbackData.BlockHeight,
		},
		AllowBatch: callbackData.AllowBatch,
	}
}

// GracefulStop On service termination, any unsent callbacks are persisted in the store, ensuring no loss of data during shutdown.
func (m *SendManager) GracefulStop() {
	if m.cancelAll != nil {
		m.cancelAll()
	}

	m.entriesWg.Wait()
}

func (m *SendManager) storeRemainingCallbacks(callbackBatch []*callbacker.CallbackEntry) {
	var err error
	// read all from callback queue and store in database
	data := make([]*store.CallbackData, len(m.callbackQueue)+len(callbackBatch))

	m.mu.Lock()
	m.callbackQueue = append(m.callbackQueue, callbackBatch...)
	for i, entry := range m.callbackQueue {
		data[i] = toStoreDto(m.url, *entry)
	}

	m.callbackQueue = m.callbackQueue[:0]
	m.mu.Unlock()

	if len(data) > 0 {
		err = m.store.SetMany(context.Background(), data)
		if err != nil {
			m.logger.Error("Failed to set remaining callbacks from queue", slog.String("err", err.Error()))
		} else {
			m.logger.Info("Stored remaining callbacks from queue", slog.Int("length", len(data)))
		}
	}

	m.entriesWg.Done()
}
