package ordered

import (
	"container/list"
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	//"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

var (
	ErrSendBatchedCallbacks = errors.New("failed to send batched callback")
)

type SendManager struct {
	url string

	// dependencies
	sender callbacker.SenderI
	store  store.CallbackerStore
	logger *slog.Logger

	expiration time.Duration

	// internal state
	entriesWg sync.WaitGroup
	cancelAll context.CancelFunc
	ctx       context.Context

	singleSendSleep   time.Duration
	batchSendInterval time.Duration
	delayDuration     time.Duration

	bufferSize   int
	callbackList *list.List
}

const (
	entriesBufferSize        = 10000
	batchSize                = 50
	defaultBatchSendInterval = 5 * time.Second
)

func WithBufferSize(size int) func(*SendManager) {
	return func(m *SendManager) {
		m.bufferSize = size
	}
}

func RunNewSendManager(url string, sender callbacker.SenderI, store store.CallbackerStore, logger *slog.Logger, sendingConfig *callbacker.SendConfig, opts ...func(*SendManager)) *SendManager {
	batchSendInterval := defaultBatchSendInterval
	if sendingConfig.BatchSendInterval != 0 {
		batchSendInterval = sendingConfig.BatchSendInterval
	}

	m := &SendManager{
		url:    url,
		sender: sender,
		store:  store,
		logger: logger,

		singleSendSleep:   sendingConfig.PauseAfterSingleModeSuccessfulSend,
		batchSendInterval: batchSendInterval,
		delayDuration:     sendingConfig.DelayDuration,
		expiration:        sendingConfig.Expiration,

		callbackList: list.New(),
		bufferSize:   entriesBufferSize,
	}

	for _, opt := range opts {
		opt(m)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	m.cancelAll = cancelAll
	m.ctx = ctx

	m.StartProcessCallbackQueue()
	return m
}

func (m *SendManager) Enqueue(entry callbacker.CallbackEntry) {
	if m.callbackList.Len() >= m.bufferSize {
		m.storeToDB(entry)
		return
	}

	m.callbackList.PushBack(entry)
}

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
	}

	return callbacks
}

func (m *SendManager) sortByTimestamp() {
	current := m.callbackList.Front()
	if m.callbackList.Front() == nil {
		return
	}
	for current != nil {
		index := current.Next()
		for index != nil {
			currentTime := current.Value.(*callbacker.CallbackEntry).Data.Timestamp
			indexTime := index.Value.(*callbacker.CallbackEntry).Data.Timestamp
			if currentTime.Before(indexTime) {
				temp := current.Value
				current.Value = index.Value
				index.Value = temp
			}
			index = index.Next()
		}
		current = current.Next()
	}
}

func (m *SendManager) capacityLeft() int {
	return m.bufferSize - m.callbackList.Len()
}

func (m *SendManager) StartProcessCallbackQueue() {
	queueTicker := time.NewTicker(m.singleSendSleep)
	batchSendTicker := time.NewTicker(m.batchSendInterval)
	sortTicker := time.NewTicker(10 * time.Second)
	backFillQueueTicker := time.NewTicker(10 * time.Second)

	m.entriesWg.Add(1)
	go func() {
		var err error
		defer func() {
			// read all from callback queue and store in database
			data := make([]*store.CallbackData, m.callbackList.Len())

			for i, entry := range m.dequeueAll() {
				data[i] = toStoreDto(m.url, entry)
			}

			err = m.store.SetMany(context.Background(), data)
			if err != nil {
				m.logger.Error("callback queue enqueue failed", slog.String("err", err.Error()))
			}
			m.entriesWg.Done()
		}()

		var callbackElements []*list.Element

	mainLoop:
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-sortTicker.C:
				m.sortByTimestamp()

			case <-backFillQueueTicker.C:
				capacityLeft := m.capacityLeft()
				if capacityLeft == 0 {
					continue
				}

				callbacks, err := m.store.PopMany(m.ctx, m.url, capacityLeft)
				if err != nil {
					m.logger.Error("Failed to load callbacks", slog.String("err", err.Error()))
					continue
				}

				for _, callback := range callbacks {
					m.Enqueue(toEntry(callback))
				}
			case <-batchSendTicker.C:
				if len(callbackElements) == 0 {
					continue
				}

				callbackBatch := make([]callbacker.CallbackEntry, len(callbackElements))
				for i, element := range callbackElements {
					cb, ok := element.Value.(callbacker.CallbackEntry)
					if !ok {
						continue
					}
					callbackBatch[i] = cb
				}

				success, retry := m.sendBatch(callbackBatch)
				if !retry || success {
					for _, callbackElement := range callbackElements {
						m.callbackList.Remove(callbackElement)
					}

					callbackElements = callbackElements[:0]
					continue
				}

				m.logger.Error("failed to send batched callbacks")
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
				if time.Until(callbackEntry.Data.Timestamp) > m.expiration {
					m.logger.Warn("callback expired", slog.Time("timestamp", callbackEntry.Data.Timestamp), slog.String("hash", callbackEntry.Data.TxID), slog.String("status", callbackEntry.Data.TxStatus))
					m.callbackList.Remove(front)
					continue
				}

				for callbackEntry.AllowBatch {
					callbackElements = append(callbackElements, front)

					callbackElement := front.Next()
					if callbackElement != nil && len(callbackElements) < batchSize {
						callbackEntry, ok = callbackElement.Value.(callbacker.CallbackEntry)
						if !ok {
							continue
						}

						continue
					}

					err = m.sendElementBatch(callbackElements)
					if err != nil {
						m.logger.Error("failed to send batched callbacks", slog.String("err", err.Error()))
					} else {
						callbackElements = callbackElements[:0]
					}
					continue mainLoop
				}

				// if entry is not a batched entry, but there are items in the batch, send them first to keep the order
				if len(callbackElements) > 0 {
					err = m.sendElementBatch(callbackElements)
					if err != nil {
						m.logger.Error("failed to send batched callbacks", slog.String("err", err.Error()))
					} else {
						callbackElements = callbackElements[:0]
					}
					continue mainLoop
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
			m.callbackList.Remove(callbackElement)
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
