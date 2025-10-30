package broadcaster_test

import (
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
)

func TestMultiKeyUTXOCreatorStart(t *testing.T) {
	t.Run("start and shutdown", func(_ *testing.T) {
		t.Parallel()
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

		// Create mocks for creators
		creators := []broadcaster.Creator{
			&mocks.CreatorMock{
				StartFunc:    func(_ uint64, _ uint64) error { return nil },
				WaitFunc:     func() {},
				ShutdownFunc: func() {},
			},
			&mocks.CreatorMock{
				StartFunc:    func(_ uint64, _ uint64) error { return errors.New("failed to start") },
				WaitFunc:     func() {},
				ShutdownFunc: func() {},
			},
		}

		// Initialize the MultiKeyUTXOCreator
		mkuc := broadcaster.NewMultiKeyUTXOCreator(logger, creators)

		// Start the MultiKeyUTXOCreator
		mkuc.Start(100, 1000)
		time.Sleep(50 * time.Millisecond)

		// Shutdown the MultiKeyUTXOCreator
		mkuc.Shutdown()
	})
}
