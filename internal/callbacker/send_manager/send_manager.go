package send_manager

import (
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

	queueProcessInterval time.Duration
	batchSendInterval    time.Duration
	batchSize            int

	now func() time.Time
}

const (
	batchSizeDefault            = 50
	queueProcessIntervalDefault = 5 * time.Second
	expirationDefault           = 24 * time.Hour
	batchSendIntervalDefault    = 5 * time.Second
	failedToSendCallbacks       = "Failed to send batch of callbacks"
	callbackElements            = "callback elements"
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

func WithQueueProcessInterval(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.queueProcessInterval = d
	}
}

func WithExpiration(d time.Duration) func(*SendManager) {
	return func(m *SendManager) {
		m.expiration = d
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

		queueProcessInterval: queueProcessIntervalDefault,
		expiration:           expirationDefault,
		batchSendInterval:    batchSendIntervalDefault,
		batchSize:            batchSizeDefault,

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
	m.storeToDB(entry)
	return
}

func (m *SendManager) Start() {
	queueTicker := time.NewTicker(m.queueProcessInterval)
	batchSendTicker := time.NewTicker(m.batchSendInterval)

	m.entriesWg.Add(1)
	var callbackBatch []*callbacker.CallbackEntry

	go func() {
		var err error
		defer m.entriesWg.Done()

		lastIterationWasBatch := false

		for {
			select {
			case <-m.ctx.Done():
				return

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
				m.logger.Debug("Batched callbacks sent on interval", slog.Int(callbackElements, len(callbackBatch)), slog.String("url", m.url))
			case <-queueTicker.C:
				callbacks, err := m.store.GetAndDelete(m.ctx, m.url, 1)
				if err != nil {
					m.logger.Error("Failed to load callbacks", slog.String("err", err.Error()))
					return
				}

				callback := callbacks[0]
				if callback == nil {
					continue
				}

				// If item is expired - dequeue without storing
				if m.now().Sub(callback.Timestamp) > m.expiration {
					m.logger.Warn("Callback expired", slog.Time("timestamp", callback.Timestamp), slog.String("hash", callback.TxID), slog.String("status", callback.TxStatus))
					continue
				}

				if callback.AllowBatch {
					lastIterationWasBatch = true

					if len(callbackBatch) < m.batchSize {

						callbackEntry := toEntry(callback)

						callbackBatch = append(callbackBatch, &callbackEntry)
						queueTicker.Reset(m.queueProcessInterval)
						continue
					}

					err = m.sendElementBatch(callbackBatch)
					if err != nil {
						m.logger.Warn(failedToSendCallbacks, slog.String("url", m.url))
						continue
					}

					callbackBatch = callbackBatch[:0]
					m.logger.Debug("Batched callbacks sent", slog.Int(callbackElements, len(callbackBatch)), slog.String("url", m.url))
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
						m.logger.Debug("Batched callbacks sent before sending single callback", slog.Int(callbackElements, len(callbackBatch)), slog.String("url", m.url))
					}
				}

				callbackEntry := toEntry(callback)

				success, retry := m.sender.Send(m.url, callbackEntry.Token, callbackEntry.Data)
				if !retry || success {
					m.logger.Debug("Single callback sent", slog.Int(callbackElements, len(callbackBatch)), slog.String("url", m.url))
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
	if retry || !success {
		dtos := toStoreDtos(m.url, callbackBatch)
		err := m.store.SetMany(m.ctx, dtos)
		if err != nil {
			return err
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

func toStoreDtos(url string, entry []callbacker.CallbackEntry) []*store.CallbackData {
	dtos := make([]*store.CallbackData, len(entry))
	for i, e := range entry {
		dtos[i] = toStoreDto(url, e)
	}

	return dtos
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
