package callbacker_test

import (
	"context"
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
		name string
	}{
		{
			name: "success",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cbStore := &mocks.ProcessorStoreMock{
				InsertFunc: func(_ context.Context, data []*store.CallbackData) (int64, error) {
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
				QueueSubscribeFunc: func(topic string, msgFunc func([]byte) error) error {
					err := msgFunc(data)
					require.NoError(t, err)
					return nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			processor, err := callbacker.NewProcessor(nil, cbStore, mqClient, logger)
			require.NoError(t, err)
			defer processor.GracefulStop()

			err = processor.Subscribe()
			require.NoError(t, err)

			processor.StartStoreCallbackRequests()

			time.Sleep(500 * time.Millisecond)
		})
	}
}
