package message_queue

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
				PublishMarshalFunc: func(_ context.Context, _ string, _ proto.Message) error {
					return tc.publishError
				},
			}

			// Create logger
			logger := slog.Default()

			// Create PublishAdapter
			adapter := NewPublishAdapter(mqClient, logger)

			// Start the publish worker
			adapter.StartPublishMarshal(tc.topic)

			// Publish test messages
			for i := 0; i < tc.messageCount; i++ {
				// Create a test proto message (using timestamppb as an example)
				msg := timestamppb.Now()
				adapter.Publish(msg)
			}

			// Give some time for messages to be processed
			time.Sleep(100 * time.Millisecond)

			// Shutdown the adapter
			adapter.Shutdown()
			require.Equal(t, tc.expectedPublished, len(mqClient.PublishMarshalCalls()))
		})
	}
}
