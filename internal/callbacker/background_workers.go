package callbacker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

var (
	ErrFailedPopMany = errors.New("failed to pop many")
)

type BackgroundWorkers struct {
	callbackerStore store.CallbackerStore
	logger          *slog.Logger
	dispatcher      *CallbackDispatcher

	workersWg sync.WaitGroup
	ctx       context.Context
	cancelAll func()
}

func NewBackgroundWorkers(s store.CallbackerStore, dispatcher *CallbackDispatcher, logger *slog.Logger) *BackgroundWorkers {
	ctx, cancel := context.WithCancel(context.Background())

	return &BackgroundWorkers{
		callbackerStore: s,
		dispatcher:      dispatcher,
		logger:          logger.With(slog.String("module", "background workers")),

		ctx:       ctx,
		cancelAll: cancel,
	}
}

func (w *BackgroundWorkers) StartCallbackStoreCleanup(interval, olderThanDuration time.Duration) {
	ctx := context.Background()
	ticker := time.NewTicker(interval)

	w.workersWg.Add(1)
	go func() {
		for {
			select {
			case <-ticker.C:
				n := time.Now()
				midnight := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
				olderThan := midnight.Add(-1 * olderThanDuration)

				err := w.callbackerStore.DeleteFailedOlderThan(ctx, olderThan)
				if err != nil {
					w.logger.Error("Failed to delete old callbacks in delay", slog.String("err", err.Error()))
				}

			case <-w.ctx.Done():
				w.workersWg.Done()
				return
			}
		}
	}()
}

func (w *BackgroundWorkers) StartFailedCallbacksDispatch(interval time.Duration) {
	const batchSize = 100

	ctx := context.Background()
	ticker := time.NewTicker(interval)

	w.workersWg.Add(1)
	go func() {
		for {
			select {
			case <-ticker.C:

				callbacks, err := w.callbackerStore.PopFailedMany(ctx, time.Now(), batchSize)
				if err != nil {
					w.logger.Error("Failed to load callbacks from store", slog.String("err", err.Error()))
					continue
				}

				if len(callbacks) == 0 {
					continue
				}
				w.logger.Info("Loaded callbacks from store", slog.Any("count", len(callbacks)))

				for _, c := range callbacks {
					callbackEntry := &CallbackEntry{
						Token:          c.Token,
						Data:           toCallback(c),
						postponedUntil: c.PostponedUntil,
					}

					w.dispatcher.Dispatch(c.URL, callbackEntry, c.AllowBatch)
				}

			case <-w.ctx.Done():
				w.workersWg.Done()
				return
			}
		}
	}()
}

func (w *BackgroundWorkers) DispatchPersistedCallbacks() error {
	w.logger.Info("Load and dispatch stored callbacks")

	const batchSize = 100
	ctx := context.Background()

	for {
		callbacks, err := w.callbackerStore.PopMany(ctx, batchSize)
		if err != nil {
			return errors.Join(ErrFailedPopMany, err)
		}

		if len(callbacks) == 0 {
			return nil
		}

		for _, c := range callbacks {
			callbackEntry := &CallbackEntry{
				Token: c.Token,
				Data:  toCallback(c),
			}

			w.dispatcher.Dispatch(c.URL, callbackEntry, c.AllowBatch)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (w *BackgroundWorkers) GracefulStop() {
	w.logger.Info("Shutting down")

	w.cancelAll()
	w.workersWg.Wait()

	w.logger.Info("Shutdown complete")
}

func toCallback(dto *store.CallbackData) *Callback {
	d := &Callback{
		Timestamp: dto.Timestamp,

		CompetingTxs: dto.CompetingTxs,
		TxID:         dto.TxID,
		TxStatus:     dto.TxStatus,
		ExtraInfo:    dto.ExtraInfo,
		MerklePath:   dto.MerklePath,

		BlockHash:   dto.BlockHash,
		BlockHeight: dto.BlockHeight,
	}

	return d
}
