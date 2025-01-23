package send_manager

import (
	"container/list"
	"context"
	"errors"
	"log/slog"
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

	bufferSize    int
	callbackQueue *list.List

	now func() time.Time
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
	ErrSendBatchedCallbacks      = errors.New("failed to send batched callback")
	ErrElementIsNotCallbackEntry = errors.New("element is not a callback entry")
)

func WithNow(nowFunc func() time.Time) func(*SendManager) {
	return func(m *SendManager) {
		m.now = nowFunc
	}
}

func WithBufferSize(size int) func(*SendManager) {
	return func(m *SendManager) {
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

		callbackQueue: list.New(),
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
	if m.callbackQueue.Len() >= m.bufferSize {
		m.storeToDB(entry)
		return
	}

	m.callbackQueue.PushBack(entry)
}

// sortByTimestampAsc sorts the callback queue by timestamp in ascending order
func (m *SendManager) sortByTimestampAsc() error {
	current := m.callbackQueue.Front()
	if m.callbackQueue.Front() == nil {
		return nil
	}
	for current != nil {
		index := current.Next()
		for index != nil {
			currentTime, ok := current.Value.(callbacker.CallbackEntry)
			if !ok {
				return ErrElementIsNotCallbackEntry
			}

			indexTime, ok := index.Value.(callbacker.CallbackEntry)
			if !ok {
				return ErrElementIsNotCallbackEntry
			}
			if currentTime.Data.Timestamp.After(indexTime.Data.Timestamp) {
				temp := current.Value
				current.Value = index.Value
				index.Value = temp
			}
			index = index.Next()
		}
		current = current.Next()
	}

	return nil
}

func (m *SendManager) CallbacksQueued() int {
	return m.callbackQueue.Len()
}

func (m *SendManager) Start() {
	queueTicker := time.NewTicker(m.queueProcessInterval)
	sortQueueTicker := time.NewTicker(m.sortByTimestampInterval)
	backfillQueueTicker := time.NewTicker(m.fillUpQueueInterval)
	batchSendTicker := time.NewTicker(m.batchSendInterval)

	m.entriesWg.Add(1)
	var callbackBatch []*list.Element

	go func() {
		var err error
		defer func() {
			// read all from callback queue and store in database
			data := make([]*store.CallbackData, m.callbackQueue.Len()+len(callbackBatch))

			for _, callbackElement := range callbackBatch {
				entry, ok := callbackElement.Value.(callbacker.CallbackEntry)
				if !ok {
					continue
				}
				m.callbackQueue.PushBack(entry)
			}

			for i, entry := range m.dequeueAll() {
				data[i] = toStoreDto(m.url, entry)
			}

			if len(data) > 0 {
				err = m.store.SetMany(context.Background(), data)
				if err != nil {
					m.logger.Error("Failed to set remaining callbacks from queue", slog.String("err", err.Error()))
				}
			}

			m.entriesWg.Done()
		}()

		lastIterationWasBatch := false

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-sortQueueTicker.C:
				err = m.sortByTimestampAsc()
				if err != nil {
					m.logger.Error("Failed to sort by timestamp", slog.String("err", err.Error()))
				}

			case <-backfillQueueTicker.C:
				m.fillUpQueue()

				m.logger.Debug("Callback queue backfilled", slog.Int("callback elements", len(callbackBatch)), slog.Int("queue length", m.CallbacksQueued()), slog.String("url", m.url))
			case <-batchSendTicker.C:
				if len(callbackBatch) == 0 {
					continue
				}

				err = m.sendElementBatch(callbackBatch)
				if err != nil {
					m.logger.Error("Failed to send batch of callbacks", slog.String("url", m.url))
					continue
				}

				callbackBatch = callbackBatch[:0]
				m.logger.Debug("Batched callbacks sent on interval", slog.Int("callback elements", len(callbackBatch)), slog.Int("queue length", m.CallbacksQueued()), slog.String("url", m.url))
			case <-queueTicker.C:
				front := m.callbackQueue.Front()
				if front == nil {
					continue
				}

				callbackEntry, ok := front.Value.(callbacker.CallbackEntry)
				if !ok {
					continue
				}

				// If item is expired - dequeue without storing
				if m.now().Sub(callbackEntry.Data.Timestamp) > m.expiration {
					m.logger.Warn("Callback expired", slog.Time("timestamp", callbackEntry.Data.Timestamp), slog.String("hash", callbackEntry.Data.TxID), slog.String("status", callbackEntry.Data.TxStatus))
					m.callbackQueue.Remove(front)
					continue
				}

				if callbackEntry.AllowBatch {
					lastIterationWasBatch = true

					if len(callbackBatch) < m.batchSize {
						callbackBatch = append(callbackBatch, front)
						queueTicker.Reset(m.queueProcessInterval)
						m.callbackQueue.Remove(front)
						continue
					}

					err = m.sendElementBatch(callbackBatch)
					if err != nil {
						m.logger.Error("Failed to send batch of callbacks", slog.String("url", m.url))
						continue
					}

					callbackBatch = callbackBatch[:0]
					m.logger.Debug("Batched callbacks sent", slog.Int("callback elements", len(callbackBatch)), slog.Int("queue length", m.CallbacksQueued()), slog.String("url", m.url))
					continue
				}

				if lastIterationWasBatch {
					lastIterationWasBatch = false
					if len(callbackBatch) > 0 {
						// if entry is not a batched entry, but last one was, send batch to keep the order
						err = m.sendElementBatch(callbackBatch)
						if err != nil {
							m.logger.Error("Failed to send batch of callbacks", slog.String("url", m.url))
							continue
						}
						callbackBatch = callbackBatch[:0]
						m.logger.Debug("Batched callbacks sent before sending single callback", slog.Int("callback elements", len(callbackBatch)), slog.Int("queue length", m.CallbacksQueued()), slog.String("url", m.url))
					}
				}

				success, retry := m.sender.Send(m.url, callbackEntry.Token, callbackEntry.Data)
				if !retry || success {
					m.callbackQueue.Remove(front)
					m.logger.Debug("Single callback sent", slog.Int("callback elements", len(callbackBatch)), slog.Int("queue length", m.CallbacksQueued()), slog.String("url", m.url))
					continue
				}
				m.logger.Error("Failed to send single callback", slog.String("url", m.url))
			}
		}
	}()
}

func (m *SendManager) sendElementBatch(callbackElements []*list.Element) error {
	var callbackElement *list.Element
	callbackBatch := make([]callbacker.CallbackEntry, 0, len(callbackElements))
	for _, element := range callbackElements {
		callback, ok := element.Value.(callbacker.CallbackEntry)
		if !ok {
			continue
		}
		callbackBatch = append(callbackBatch, callback)
	}
	success, retry := m.sendBatch(callbackBatch)
	if !retry || success {
		for _, callbackElement = range callbackElements {
			m.callbackQueue.Remove(callbackElement)
		}

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
	capacityLeft := m.bufferSize - m.callbackQueue.Len()
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

func (m *SendManager) dequeueAll() []callbacker.CallbackEntry {
	callbacks := make([]callbacker.CallbackEntry, 0, m.callbackQueue.Len())

	var next *list.Element
	for front := m.callbackQueue.Front(); front != nil; front = next {
		next = front.Next()
		entry, ok := front.Value.(callbacker.CallbackEntry)
		if !ok {
			m.callbackQueue.Remove(front)
			continue
		}
		callbacks = append(callbacks, entry)

		m.callbackQueue.Remove(front)
	}

	return callbacks
}

// GracefulStop On service termination, any unsent callbacks are persisted in the store, ensuring no loss of data during shutdown.
func (m *SendManager) GracefulStop() {
	if m.cancelAll != nil {
		m.cancelAll()
	}

	m.entriesWg.Wait()
}
