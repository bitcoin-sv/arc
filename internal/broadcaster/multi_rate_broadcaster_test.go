package broadcaster_test

import (
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/stretchr/testify/require"
)

func TestMultiRateBroadcasterStart(t *testing.T) {

	tt := []struct {
		name          string
		startErr      error
		expectedError string
	}{
		{
			name: "start and shutdown successfully",
		},
		{
			name:          "error - failed to start",
			startErr:      errors.New("failed to start"),
			expectedError: "failed to start",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

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
				StartFunc:              func() error { return tc.startErr },
				WaitFunc:               func() {},
				ShutdownFunc:           func() {},
				GetTxCountFunc:         func() int64 { return 10 },
				GetConnectionCountFunc: func() int64 { return 1 },
				GetLimitFunc:           func() int64 { return 200 },
				GetUtxoSetLenFunc:      func() int { return 1000 },
			}

			mcs := broadcaster.NewMultiKeyRateBroadcaster(logger, []broadcaster.RateBroadcaster{rateBroadcaster1, rateBroadcaster2})

			err := mcs.Start()
			defer mcs.Shutdown()

			if tc.expectedError != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedError)
				return
			} else {
				require.NoError(t, err)
			}

			time.Sleep(50 * time.Millisecond)
		})
	}
}
