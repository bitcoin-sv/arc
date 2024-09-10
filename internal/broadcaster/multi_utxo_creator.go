package broadcaster

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type MultiKeyUTXOCreator struct {
	creators    []*UTXOCreator
	logger      *slog.Logger
	cancelAll   context.CancelFunc
	ctx         context.Context
	wg          sync.WaitGroup
	logInterval time.Duration
}

// NewMultiKeyUTXOCreator initializes the MultiKeyUTXOCreator.
func NewMultiKeyUTXOCreator(logger *slog.Logger, creators []*UTXOCreator, opts ...func(p *MultiKeyUTXOCreator)) *MultiKeyUTXOCreator {
	ctx, cancelAll := context.WithCancel(context.Background())
	mkuc := &MultiKeyUTXOCreator{
		creators:    creators,
		logger:      logger,
		ctx:         ctx,
		cancelAll:   cancelAll,
		logInterval: 2 * time.Second,
	}

	for _, opt := range opts {
		opt(mkuc)
	}

	return mkuc
}

func (mkuc *MultiKeyUTXOCreator) Start(outputs int, satoshisPerOutput uint64) {

	for _, creator := range mkuc.creators {
		creator := creator
		mkuc.wg.Add(1)
		go func() {
			defer mkuc.wg.Done()
			if err := creator.CreateUtxos(outputs, satoshisPerOutput); err != nil {
				mkuc.logger.Error("failed to create UTXOs", slog.String("error", err.Error()))
			}
		}()
	}

	mkuc.wg.Wait()
}

func (mkuc *MultiKeyUTXOCreator) Shutdown() {
	mkuc.cancelAll()
	for _, creator := range mkuc.creators {
		creator.Shutdown()
	}
	mkuc.wg.Wait()
}
