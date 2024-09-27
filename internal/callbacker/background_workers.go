package callbacker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type BackgroundWorkers struct {
	s store.CallbackerStore
	l *slog.Logger
	d *CallbackDispatcher

	workersWg sync.WaitGroup
	ctx       context.Context
	cancelAll func()
}

func NewBackgroundWorkers(store store.CallbackerStore, dispatcher *CallbackDispatcher, logger *slog.Logger) *BackgroundWorkers {
	ctx, cancel := context.WithCancel(context.Background())

	return &BackgroundWorkers{
		s: store,
		d: dispatcher,
		l: logger.With(slog.String("module", "background workers")),

		ctx:       ctx,
		cancelAll: cancel,
	}
}

func (w *BackgroundWorkers) StartCallbackStoreCleanup(interval, olderThanDuration time.Duration) {
	w.workersWg.Add(1)
	go w.pruneCallbacks(interval, olderThanDuration)
}

func (w *BackgroundWorkers) StartQuarantineCallbacksDispatch(interval time.Duration) {
	w.workersWg.Add(1)
	go w.dispatchQuarantineCallbacks(interval)
}

func (w *BackgroundWorkers) GracefulStop() {
	w.cancelAll()
	w.workersWg.Wait()
}

func (w *BackgroundWorkers) pruneCallbacks(interval, olderThanDuration time.Duration) {
	ctx := context.Background()
	t := time.NewTicker(interval)

	for {
		select {
		case <-t.C:
			n := time.Now()
			midnight := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
			olderThan := midnight.Add(-1 * olderThanDuration)

			err := w.s.DeleteFailedOlderThan(ctx, olderThan)
			if err != nil {
				w.l.Error("failed to delete old callbacks in quarantine", slog.String("err", err.Error()))
			}

		case <-w.ctx.Done():
			w.workersWg.Done()
			return
		}
	}
}

func (w *BackgroundWorkers) dispatchQuarantineCallbacks(interval time.Duration) {
	const batchSize = 100

	ctx := context.Background()
	t := time.NewTicker(interval)

	for {
		select {
		case <-t.C:
			callbacks, err := w.s.PopFailedMany(ctx, time.Now(), batchSize)
			if err != nil {
				w.l.Error("reading callbacks from store failed", slog.String("err", err.Error()))
				continue
			}

			if len(callbacks) == 0 {
				continue
			}

			for _, c := range callbacks {
				w.d.Dispatch(c.Url, toCallbackEntry(c), c.AllowBatch)
			}

		case <-w.ctx.Done():
			w.workersWg.Done()
			return
		}
	}
}

func toCallbackEntry(dto *store.CallbackData) *CallbackEntry {
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

	return &CallbackEntry{
		Token:          dto.Token,
		Data:           d,
		postponedUntil: dto.PostponedUntil,
	}
}
