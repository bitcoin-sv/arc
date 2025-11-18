package metamorph

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestPublishAdapter_StartPublishMarshal(t *testing.T) {
	tt := []struct {
		name             string
		topic            string
		messageCount     int
		publishError     error
		publishCoreError error

		expectedPublished     int
		expectedPublishedCore int
	}{
		{
			name:         "successfully publishes single message",
			topic:        "test-topic",
			messageCount: 1,
			publishError: nil,

			expectedPublished:     2,
			expectedPublishedCore: 1,
		},
		{
			name:         "successfully publishes multiple messages",
			topic:        "test-topic",
			messageCount: 5,
			publishError: nil,

			expectedPublished:     10,
			expectedPublishedCore: 5,
		},
		{
			name:         "handles publish error gracefully",
			topic:        "test-topic",
			messageCount: 3,
			publishError: errors.New("some error"),

			expectedPublished:     6,
			expectedPublishedCore: 3,
		},
		{
			name:             "handles publish core error gracefully",
			topic:            "test-topic",
			messageCount:     3,
			publishCoreError: errors.New("some error"),

			expectedPublished:     6,
			expectedPublishedCore: 3,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Create mock MQ client
			mqClient := &mqMocks.MessageQueueClientMock{
				PublishMarshalFunc: func(_ context.Context, _ string, _ proto.Message) error { return tc.publishError },
				PublishCoreFunc:    func(_ string, _ []byte) error { return tc.publishCoreError },
			}

			// Create logger
			logger := slog.Default()

			// Create PublishAdapter
			adapter := NewPublishAdapter(mqClient, logger)
			callbackerChan := make(chan *callbacker_api.SendRequest, 10)
			registerTxsChan := make(chan *blocktx_api.Transactions, 10)
			registerTxChan := make(chan []byte, 10)

			// Start the publish worker
			adapter.StartPublishBlockTransactions(tc.topic, registerTxsChan)
			adapter.StartPublishSendRequests(tc.topic, callbackerChan)
			adapter.StartPublishCore(tc.topic, registerTxChan)

			// Publish test messages
			for i := 0; i < tc.messageCount; i++ {
				// Create a test proto message (using timestamppb as an example)
				callbackerChan <- &callbacker_api.SendRequest{}
				registerTxsChan <- &blocktx_api.Transactions{}
				registerTxChan <- []byte("test-message")
			}

			// Give some time for messages to be processed
			time.Sleep(100 * time.Millisecond)

			// Shutdown the adapter
			adapter.Shutdown()
			assert.Equal(t, tc.expectedPublished, len(mqClient.PublishMarshalCalls()))
			assert.Equal(t, tc.expectedPublishedCore, len(mqClient.PublishCoreCalls()))
		})
	}
}
