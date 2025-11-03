package callbacker_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/stretchr/testify/require"
)

func TestProcessor_StartStoreCallbackRequests(t *testing.T) {
	tt := []struct {
		name                   string
		storeCallbackBatchSize int
		storeCallbacksInterval time.Duration

		expectedError       error
		expectedInsertCalls int
	}{
		{
			name:                   "insert batch",
			storeCallbackBatchSize: 4,
			storeCallbacksInterval: 20 * time.Second,

			expectedInsertCalls: 1,
		},
		{
			name:                   "insert in interval",
			storeCallbackBatchSize: 50,
			storeCallbacksInterval: 20 * time.Millisecond,

			expectedInsertCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cbStore := &mocks.ProcessorStoreMock{
				InsertFunc: func(_ context.Context, _ []*store.CallbackData) (int64, error) {
					return 4, nil
				},
			}

			msg := &callbacker_api.SendRequest{
				CallbackRouting: &callbacker_api.CallbackRouting{
					Url:        "abc.com",
					Token:      "1234",
					AllowBatch: false,
				},
				Txid:         "xyz",
				Status:       callbacker_api.Status_MINED,
				MerklePath:   "",
				ExtraInfo:    "",
				CompetingTxs: nil,
				BlockHash:    "",
				BlockHeight:  0,
				Timestamp:    nil,
			}

			sendRequestCh := make(chan *callbacker_api.SendRequest, 20)
			for range 5 {
				sendRequestCh <- msg
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			processor, err := callbacker.NewProcessor(nil,
				cbStore,
				sendRequestCh,
				logger,
				callbacker.WithStoreCallbackBatchSize(tc.storeCallbackBatchSize),
				callbacker.WithStoreCallbacksInterval(tc.storeCallbacksInterval),
			)
			require.NoError(t, err)
			defer processor.Shutdown()

			require.NoError(t, err)

			processor.StartStoreCallbackRequests()

			time.Sleep(500 * time.Millisecond)

			require.Equal(t, tc.expectedInsertCalls, len(cbStore.InsertCalls()))
		})
	}
}
