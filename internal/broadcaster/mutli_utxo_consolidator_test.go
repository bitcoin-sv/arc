package broadcaster_test

import (
	"errors"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestMultiKeyUtxoConsolidatorStart(t *testing.T) {

	t.Run("start and shutdown", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

		cs := []broadcaster.Consolidator{
			&mocks.ConsolidatorMock{
				StartFunc:    func() error { return nil },
				WaitFunc:     func() {},
				ShutdownFunc: func() {},
			},
			&mocks.ConsolidatorMock{
				StartFunc:    func() error { return errors.New("failed to start") },
				WaitFunc:     func() {},
				ShutdownFunc: func() {},
			},
		}

		mcs := broadcaster.NewMultiKeyUtxoConsolidator(logger, cs)
		mcs.Start()
		time.Sleep(50 * time.Millisecond)
		mcs.Shutdown()

	})
}
