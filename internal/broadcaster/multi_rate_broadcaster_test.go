package broadcaster_test

import (
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
)

func TestMultiRateBroadcasterStart(t *testing.T) {
	tt := []struct {
		name          string
		expectedError error
	}{
		{
			name: "start and shutdown successfully",
		},
		{
			name:          "error - failed to start",
			expectedError: errors.New("failed to start"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			rateBroadcaster1 := &mocks.RateBroadcasterMock{
				StartFunc:              func() error { return nil },
				WaitFunc:               func() {},
				ShutdownFunc:           func() {},
				GetTxCountFunc:         func() int64 { return 5 },
				GetConnectionCountFunc: func() int64 { return 2 },
				GetLimitFunc:           func() int64 { return 100 },
				GetUtxoSetLenFunc:      func() int { return 1000 },
			}

			rateBroadcaster2 := &mocks.RateBroadcasterMock{
				StartFunc:              func() error { return tc.expectedError },
				WaitFunc:               func() {},
				ShutdownFunc:           func() {},
				GetTxCountFunc:         func() int64 { return 10 },
				GetConnectionCountFunc: func() int64 { return 1 },
				GetLimitFunc:           func() int64 { return 200 },
				GetUtxoSetLenFunc:      func() int { return 1000 },
			}

			sut := broadcaster.NewMultiKeyRateBroadcaster(logger, []broadcaster.RateBroadcaster{rateBroadcaster1, rateBroadcaster2},
				broadcaster.WithLogInterval(1*time.Millisecond))

			// when
			actualError := sut.Start()
			defer sut.Shutdown()

			// then
			if actualError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				return
			}
			require.NoError(t, actualError)

			time.Sleep(50 * time.Millisecond)
		})
	}
}
