package ordered

import (
	"container/list"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type SendManagerStore interface {
	Set(ctx context.Context, dto *store.CallbackData) error
	SetMany(ctx context.Context, data []*store.CallbackData) error
	PopMany(ctx context.Context, url string, limit int) ([]*store.CallbackData, error)
}

type SendManager struct {
	url string

	// dependencies
	sender callbacker.SenderI
	store  SendManagerStore
	logger *slog.Logger

	expiration time.Duration

	// internal state
	entriesWg sync.WaitGroup
	cancelAll context.CancelFunc
	ctx       context.Context

	singleSendInterval time.Duration
	//batchSendInterval time.Duration
	//delayDuration     time.Duration

	bufferSize   int
	callbackList *list.List

	now func() time.Time
}

const (
	entriesBufferSize         = 10000
	batchSendIntervalDefault  = 5 * time.Second
	singleSendIntervalDefault = 5 * time.Second
	expirationDefault         = 24 * time.Hour
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

func WithSingleSendInterval(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.singleSendInterval = d
	}
}

func WithExpiration(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.expiration = d
	}
}

func New(url string, sender callbacker.SenderI, store SendManagerStore, logger *slog.Logger, opts ...func(*SendManager)) *SendManager {
	//batchSendInterval := defaultBatchSendInterval
	//if sendingConfig.BatchSendInterval != 0 {
	//	batchSendInterval = sendingConfig.BatchSendInterval
	//}

	m := &SendManager{
		url:    url,
		sender: sender,
		store:  store,
		logger: logger,

		singleSendInterval: singleSendIntervalDefault,
		//batchSendInterval: batchSendInterval,
		//delayDuration:     sendingConfig.DelayDuration,
		expiration: expirationDefault,

		callbackList: list.New(),
		bufferSize:   entriesBufferSize,
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
	if m.callbackList.Len() >= m.bufferSize {
		m.storeToDB(entry)
		return
	}

	m.callbackList.PushBack(entry)
}

//func (m *SendManager) sortByTimestamp() {
//	current := m.callbackList.Front()
//	if m.callbackList.Front() == nil {
//		return
//	}
//	for current != nil {
//		index := current.Next()
//		for index != nil {
//			currentTime := current.Value.(*callbacker.CallbackEntry).Data.Timestamp
//			indexTime := index.Value.(*callbacker.CallbackEntry).Data.Timestamp
//			if currentTime.Before(indexTime) {
//				temp := current.Value
//				current.Value = index.Value
//				index.Value = temp
//			}
//			index = index.Next()
//		}
//		current = current.Next()
//	}
//}

func (m *SendManager) CallbacksQueued() int {
	return m.callbackList.Len()
}

func (m *SendManager) Start() {
	queueTicker := time.NewTicker(m.singleSendInterval)
	//sortTicker := time.NewTicker(10 * time.Second)
	//backFillQueueTicker := time.NewTicker(10 * time.Second)

	m.entriesWg.Add(1)
	go func() {
		var err error
		defer func() {
			// read all from callback queue and store in database
			data := make([]*store.CallbackData, m.callbackList.Len())

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

		for {
			select {
			case <-m.ctx.Done():
				return
			//case <-sortTicker.C:
			//	m.sortByTimestamp()

			//case <-backFillQueueTicker.C:
			//	capacityLeft := m.bufferSize - m.callbackList.Len()
			//	if capacityLeft == 0 {
			//		continue
			//	}
			//
			//	callbacks, err := m.store.PopMany(m.ctx, m.url, capacityLeft)
			//	if err != nil {
			//		m.logger.Error("Failed to load callbacks", slog.String("err", err.Error()))
			//		continue
			//	}
			//
			//	for _, callback := range callbacks {
			//		m.Enqueue(toEntry(callback))
			//	}

			case <-queueTicker.C:
				front := m.callbackList.Front()
				if front == nil {
					continue
				}

				callbackEntry, ok := front.Value.(callbacker.CallbackEntry)
				if !ok {
					continue
				}

				// If item is expired - dequeue without storing
				if m.now().Sub(callbackEntry.Data.Timestamp) > m.expiration {
					m.logger.Warn("callback expired", slog.Time("timestamp", callbackEntry.Data.Timestamp), slog.String("hash", callbackEntry.Data.TxID), slog.String("status", callbackEntry.Data.TxStatus))
					m.callbackList.Remove(front)
					continue
				}

				success, retry := m.sender.Send(m.url, callbackEntry.Token, callbackEntry.Data)
				if !retry || success {
					m.callbackList.Remove(front)
					continue
				}
				m.logger.Error("failed to send single callback", slog.String("url", m.url))
			}
		}
	}()
}

func (m *SendManager) storeToDB(entry callbacker.CallbackEntry) {
	callbackData := toStoreDto(m.url, entry)
	err := m.store.Set(context.Background(), callbackData)
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

//
//func toEntry(callbackData *store.CallbackData) callbacker.CallbackEntry {
//	return callbacker.CallbackEntry{
//		Token: callbackData.Token,
//		Data: &callbacker.Callback{
//			Timestamp:    callbackData.Timestamp,
//			CompetingTxs: callbackData.CompetingTxs,
//			TxID:         callbackData.TxID,
//			TxStatus:     callbackData.TxStatus,
//			ExtraInfo:    callbackData.ExtraInfo,
//			MerklePath:   callbackData.MerklePath,
//			BlockHash:    callbackData.BlockHash,
//			BlockHeight:  callbackData.BlockHeight,
//		},
//		AllowBatch: callbackData.AllowBatch,
//	}
//}

func (m *SendManager) dequeueAll() []callbacker.CallbackEntry {
	callbacks := make([]callbacker.CallbackEntry, 0, m.callbackList.Len())

	var next *list.Element
	for front := m.callbackList.Front(); front != nil; front = next {
		next = front.Next()
		entry, ok := front.Value.(callbacker.CallbackEntry)
		if !ok {
			continue
		}
		callbacks = append(callbacks, entry)

		m.callbackList.Remove(front)
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
