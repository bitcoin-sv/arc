package blocktx

import (
	"context"
	"log/slog"
	"sync"
	"time"


	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/p2p"
)

type BackgroundWorkers struct {
	logger *slog.Logger
	store  store.BlocktxStore

	workersWg sync.WaitGroup
	ctx       context.Context
	cancelAll func()
}

func NewBackgroundWorkers(store store.BlocktxStore, logger *slog.Logger) *BackgroundWorkers {
	ctx, cancel := context.WithCancel(context.Background())

	return &BackgroundWorkers{
		store:  store,
		logger: logger.With(slog.String("module", "background workers")),

		ctx:       ctx,
		cancelAll: cancel,
	}
}

func (w *BackgroundWorkers) GracefulStop() {
	w.logger.Info("Shutting down")

	w.cancelAll()
	w.workersWg.Wait()

	w.logger.Info("Shutdown complete")
}

func (w *BackgroundWorkers) StartFillGaps(peers []p2p.PeerI, interval time.Duration, retentionDays int, blockRequestingCh chan<- blocktx_p2p.BlockRequest) {
	w.workersWg.Add(1)

	go func() {
		defer w.workersWg.Done()

		ticker := time.NewTicker(interval)
		i := 0

		for {
			select {
			case <-ticker.C:
				i = i % len(peers)
				err := w.fillGaps(peers[i], retentionDays, blockRequestingCh)
				if err != nil {
					w.logger.Error("failed to fill blocks gaps", slog.String("err", err.Error()))
				}

				i++
				ticker.Reset(interval)

			case <-w.ctx.Done():
				return
			}
		}
	}()
}

func (w *BackgroundWorkers) fillGaps(peer p2p.PeerI, retentionDays int, blockRequestingCh chan<- blocktx_p2p.BlockRequest) error {
	const (
		hoursPerDay   = 24
		blocksPerHour = 6
	)

	heightRange := retentionDays * hoursPerDay * blocksPerHour
	blockHeightGaps, err := w.store.GetBlockGaps(w.ctx, heightRange)
	if err != nil || len(blockHeightGaps) == 0 {
		return err
	}

	for i, block := range blockHeightGaps {
		if i == maxRequestBlocks {
			break
		}

		w.logger.Info("adding request for missing block to request channel",
			slog.String("hash", block.Hash.String()),
			slog.Uint64("height", block.Height),
			slog.String("peer", peer.String()),
		)

		blockRequestingCh <- blocktx_p2p.BlockRequest{
			Hash: block.Hash,
			Peer: peer,
		}
	}

	return nil
}
