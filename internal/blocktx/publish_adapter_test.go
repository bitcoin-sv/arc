package blocktx

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestPublishAdapter_StartPublishMarshal(t *testing.T) {
	tt := []struct {
		name         string
		topic        string
		messageCount int
		publishError error

		expectedPublished int
	}{
		{
			name:         "successfully publishes single message",
			topic:        "test-topic",
			messageCount: 1,
			publishError: nil,

			expectedPublished: 1,
		},
		{
			name:         "successfully publishes multiple messages",
			topic:        "test-topic",
			messageCount: 5,
			publishError: nil,

			expectedPublished: 5,
		},
		{
			name:         "handles publish error gracefully",
			topic:        "test-topic",
			messageCount: 3,
			publishError: errors.New("some error"),

			expectedPublished: 3,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Create mock MQ client
			mqClient := &mqMocks.MessageQueueClientMock{
				PublishMarshalCoreFunc: func(_ string, _ proto.Message) error {
					return tc.publishError
				},
			}

			// Create logger
			logger := slog.Default()

			// Create PublishAdapter
			adapter := NewPublishAdapter(mqClient, logger)
			minedTxsChan := make(chan *blocktx_api.TransactionBlocks, 10)
			// Start the publish worker
			adapter.StartPublishMarshal(tc.topic, minedTxsChan)

			// Publish test messages
			for i := 0; i < tc.messageCount; i++ {
				// Create a test proto message (using timestamppb as an example)
				minedTxsChan <- &blocktx_api.TransactionBlocks{}
			}

			// Give some time for messages to be processed
			time.Sleep(100 * time.Millisecond)

			// Shutdown the adapter
			adapter.Shutdown()
			require.Equal(t, tc.expectedPublished, len(mqClient.PublishMarshalCoreCalls()))
		})
	}
}
