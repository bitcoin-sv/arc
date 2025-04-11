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
	SetMany(ctx context.Context, data []*store.CallbackData) error
	GetAndDeleteTx(ctx context.Context, url string, limit int, expiration time.Duration, batch bool) (data []*store.CallbackData, commitFunc func() error, rollbackFunc func() error, err error)
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
	storeChan            chan callbacker.CallbackEntry
}

const (
	batchSizeDefault            = 50
	queueProcessIntervalDefault = 5 * time.Second
	expirationDefault           = 24 * time.Hour
	batchSendIntervalDefault    = 5 * time.Second
)

var (
	ErrSendBatchedCallbacks      = errors.New("failed to send batched callback")
	ErrElementIsNotCallbackEntry = errors.New("element is not a callback entry")
)

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
		storeChan:            make(chan callbacker.CallbackEntry, 5),
	}

	for _, opt := range opts {
		opt(m)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	m.cancelAll = cancelAll
	m.ctx = ctx

	return m
}

func (m *SendManager) StartStore() {
	const (
		storeCallbacksInterval       = 5 * time.Second
		failedToStoreCallbacksErrMsg = "Failed to store callbacks"
		storeCallbackBatchSize       = 20
	)

	storeTicker := time.NewTicker(storeCallbacksInterval)

	go func() {
		var toStore []*store.CallbackData
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-storeTicker.C:
				if len(toStore) > 0 {
					err := m.store.SetMany(m.ctx, toStore)
					if err != nil {
						m.logger.Error(failedToStoreCallbacksErrMsg, slog.String("err", err.Error()))
						continue
					}

					toStore = toStore[:0]
				}
			case entry := <-m.storeChan:
				toStore = append(toStore, toStoreDto(m.url, entry))

				if len(toStore) >= storeCallbackBatchSize {
					err := m.store.SetMany(m.ctx, toStore)
					if err != nil {
						m.logger.Error(failedToStoreCallbacksErrMsg, slog.String("err", err.Error()))
					}
				}
			}
		}
	}()
}

func (m *SendManager) Enqueue(entry callbacker.CallbackEntry) {
	m.storeChan <- entry
}

func (m *SendManager) Start() {
	queueTicker := time.NewTicker(m.queueProcessInterval)
	batchSendTicker := time.NewTicker(m.batchSendInterval)

	m.entriesWg.Add(1)

	go func() {
		defer m.entriesWg.Done()

		for {
			select {
			case <-m.ctx.Done():
				return

			case <-batchSendTicker.C:
				callbacks, commit, rollback, err := m.store.GetAndDeleteTx(m.ctx, m.url, m.batchSize, m.expiration, true)
				if err != nil {
					m.logger.Error("Failed to load callbacks", slog.String("err", err.Error()))
					return
				}
				callbackBatch := make([]*callbacker.CallbackEntry, len(callbacks))

				for i, callback := range callbacks {
					callbackEntry := toEntry(callback)
					callbackBatch[i] = &callbackEntry
				}

				err = m.sendElementBatch(callbackBatch)
				if err != nil {
					m.logger.Warn("Failed to send batch of callbacks", slog.String("url", m.url))

					err = rollback()
					if err != nil {
						m.logger.Warn("Failed to rollback batched callback deletion", slog.String("url", m.url), slog.String("err", err.Error()))
					}
					continue
				}

				m.logger.Debug("Batched callbacks sent on interval", slog.Int("callback elements", len(callbackBatch)), slog.String("url", m.url))

				err = commit()
				if err != nil {
					m.logger.Warn("Failed to commit batched callback deletion", slog.String("url", m.url))
				}
			case <-queueTicker.C:
				callbacks, commit, rollback, err := m.store.GetAndDeleteTx(m.ctx, m.url, 1, m.expiration, false)
				if err != nil {
					m.logger.Error("Failed to load callbacks", slog.String("err", err.Error()))
					return
				}

				callback := callbacks[0]

				callbackEntry := toEntry(callback)
				success, retry := m.sender.Send(m.url, callbackEntry.Token, callbackEntry.Data)
				if !retry || success {
					m.logger.Debug("Single callback sent", slog.String("url", m.url))

					err = commit()
					if err != nil {
						m.logger.Error("Failed to commit callback", slog.String("err", err.Error()))
					}

					continue
				}

				err = rollback()
				if err != nil {
					m.logger.Error("Failed to rollback callback", slog.String("err", err.Error()))
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
