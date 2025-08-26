package callbacker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
)

func TestStartCallbackStoreCleanup(t *testing.T) {
	tt := []struct {
		name                     string
		deleteFailedOlderThanErr error

		expectedIterations int
	}{
		{
			name: "success",

			expectedIterations: 4,
		},
		{
			name:                     "error deleting failed older than",
			deleteFailedOlderThanErr: errors.New("some error"),

			expectedIterations: 4,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cbStore := &mocks.ProcessorStoreMock{
				ClearFunc: func(_ context.Context, _ time.Time) error {
					return tc.deleteFailedOlderThanErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			processor, err := callbacker.NewProcessor(nil, cbStore, nil, logger)
			require.NoError(t, err)
			defer processor.GracefulStop()

			processor.StartCallbackStoreCleanup(20*time.Millisecond, 50*time.Second)

			time.Sleep(90 * time.Millisecond)

			require.Equal(t, tc.expectedIterations, len(cbStore.ClearCalls()))
		})
	}
}
