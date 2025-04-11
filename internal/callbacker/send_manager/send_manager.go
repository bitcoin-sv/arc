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

	singleSendInterval time.Duration
	batchSendInterval  time.Duration
	batchSize          int
	storeChan          chan callbacker.CallbackEntry
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
	logger = logger.With("url", url)

	m := &SendManager{
		url:    url,
		sender: sender,
		store:  store,
		logger: logger,

		singleSendInterval: queueProcessIntervalDefault,
		expiration:         expirationDefault,
		batchSendInterval:  batchSendIntervalDefault,
		batchSize:          batchSizeDefault,
		storeChan:          make(chan callbacker.CallbackEntry, 5),
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

func (m *SendManager) Enqueue(entry callbacker.CallbackEntry) {
	m.storeChan <- entry
}

func (m *SendManager) Start() {
	singleSendTicker := time.NewTicker(m.singleSendInterval)
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

				callbackBatch := make([]*callbacker.Callback, len(callbacks))
				for i, callback := range callbacks {
					callbackEntry := toEntry(callback)
					callbackBatch[i] = callbackEntry.Data
				}
				success, retry := m.sender.SendBatch(m.url, callbacks[0].Token, callbackBatch)
				if !retry || success {
					if success {
						m.logger.Debug("Batched callbacks sent", slog.Int("callback elements", len(callbackBatch)))
					}

					err = commit()
					if err != nil {
						m.logger.Warn("Failed to commit batched callback deletion")
					}

					continue
				}

				err = rollback()
				if err != nil {
					m.logger.Warn("Failed to rollback batched callback deletion", slog.String("err", err.Error()))
				}
				m.logger.Warn("Failed to send batch of callbacks")

			case <-singleSendTicker.C:
				callbacks, commit, rollback, err := m.store.GetAndDeleteTx(m.ctx, m.url, 1, m.expiration, false)
				if err != nil {
					m.logger.Error("Failed to load callbacks", slog.String("err", err.Error()))
					return
				}

				callbackEntry := toEntry(callbacks[0])
				success, retry := m.sender.Send(m.url, callbackEntry.Token, callbackEntry.Data)
				if !retry || success {
					if success {
						m.logger.Debug("Single callback sent")
					}

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

				m.logger.Warn("Failed to send single callback")
			}
		}
	}()
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
