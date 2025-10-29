package broadcaster_test

import (
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
)

func TestMultiKeyUtxoConsolidatorStart(t *testing.T) {
	testutils.RunParallel(t, true, "start and shutdown", func(_ *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

		cs := []broadcaster.Consolidator{
			&mocks.ConsolidatorMock{
				StartFunc:    func(_ int) error { return nil },
				WaitFunc:     func() {},
				ShutdownFunc: func() {},
			},
			&mocks.ConsolidatorMock{
				StartFunc:    func(_ int) error { return errors.New("failed to start") },
				WaitFunc:     func() {},
				ShutdownFunc: func() {},
			},
		}

		sut := broadcaster.NewMultiKeyUtxoConsolidator(logger, cs)

		sut.Start()
		time.Sleep(50 * time.Millisecond)
		sut.Shutdown()
	})
}
