package callbacker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
)

func TestProcessor_StartStoreCallbackRequests(t *testing.T) {
	tt := []struct {
		name                   string
		storeCallbackBatchSize int
		storeCallbacksInterval time.Duration
		consumeErr             error

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
		{
			name:                   "insert batch - consume err",
			storeCallbackBatchSize: 4,
			storeCallbacksInterval: 20 * time.Second,
			consumeErr:             errors.New("some error"),

			expectedError:       callbacker.ErrConsume,
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
			data, err := proto.Marshal(msg)
			require.NoError(t, err)

			mqClient := &mqMocks.MessageQueueClientMock{
				ConsumeFunc: func(_ string, msgFunc func([]byte) error) error {
					if tc.consumeErr != nil {
						return tc.consumeErr
					}
					for range 5 {
						err := msgFunc(data)
						require.NoError(t, err)
					}
					return nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			processor, err := callbacker.NewProcessor(nil,
				cbStore,
				mqClient,
				logger,
				callbacker.WithStoreCallbackBatchSize(tc.storeCallbackBatchSize),
				callbacker.WithStoreCallbacksInterval(tc.storeCallbacksInterval),
			)
			require.NoError(t, err)
			defer processor.GracefulStop()

			err = processor.Subscribe()
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			processor.StartStoreCallbackRequests()

			time.Sleep(500 * time.Millisecond)

			require.Equal(t, tc.expectedInsertCalls, len(cbStore.InsertCalls()))
		})
	}
}
