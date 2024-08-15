package broadcaster_test

import (
	"errors"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
)

func TestMultiRateBroadcasterStart(t *testing.T) {

	tt := []struct {
		name     string
		startErr error

		expectedErrorStr string
	}{
		{
			name: "start and shutdown",
		},
		{
			name:     "start and shutdown",
			startErr: errors.New("failed to start"),

			expectedErrorStr: "failed to start",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			cs := []broadcaster.RateBroadcaster{
				&mocks.RateBroadcasterMock{
					StartFunc:              func() error { return nil },
					WaitFunc:               func() {},
					ShutdownFunc:           func() {},
					GetTxCountFunc:         func() int64 { return 5 },
					GetConnectionCountFunc: func() int64 { return 2 },
					GetLimitFunc:           func() int64 { return 100 },
					GetKeyNameFunc:         func() string { return "key-01" },
					GetUtxoSetLenFunc:      func() int { return 1000 },
				},
				&mocks.RateBroadcasterMock{
					StartFunc:              func() error { return tc.startErr },
					WaitFunc:               func() {},
					ShutdownFunc:           func() {},
					GetTxCountFunc:         func() int64 { return 10 },
					GetConnectionCountFunc: func() int64 { return 1 },
					GetLimitFunc:           func() int64 { return 200 },
					GetKeyNameFunc:         func() string { return "key-02" },
					GetUtxoSetLenFunc:      func() int { return 1000 },
				},
			}

			mcs := broadcaster.NewMultiKeyRateBroadcaster(logger, cs, broadcaster.WithLogInterval(20*time.Millisecond))
			err := mcs.Start()

			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			time.Sleep(50 * time.Millisecond)
			mcs.Shutdown()
		})
	}

}
