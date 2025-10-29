package callbacker

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestPublishAdapter_StartPublishMarshal(t *testing.T) {
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

	wrongMsg := []byte("wrong msg")

	tt := []struct {
		name       string
		consumeErr error
		data       []byte

		expectedError     error
		expectedCallbacks int
	}{
		{
			name: "success",
			data: data,

			expectedCallbacks: 5,
		},
		{
			name: "error - wrong data",
			data: wrongMsg,

			expectedCallbacks: 0,
		},
		{
			name: "no data",
			data: nil,

			expectedCallbacks: 0,
		},
		{
			name:       "error - consume",
			consumeErr: errors.New("some error"),
			data:       data,

			expectedError:     ErrConsume,
			expectedCallbacks: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create mock MQ client
			mqClient := &mqMocks.MessageQueueClientMock{
				ConsumeFunc: func(_ string, msgFunc func([]byte) error) error {
					if tc.consumeErr != nil {
						return tc.consumeErr
					}
					for range 5 {
						_ = msgFunc(tc.data)
					}
					return nil
				},
			}

			// Create logger
			logger := slog.Default()

			adapter := NewMessageSubscribeAdapter(mqClient, logger)
			defer adapter.Shutdown()

			sendRequestCh := make(chan *callbacker_api.SendRequest, 10)

			actualError := adapter.Start(sendRequestCh)
			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				return
			}
			require.NoError(t, actualError)
			callbacks := 0
		registerLoop:
			for {
				select {
				case <-sendRequestCh:
					callbacks++
				case <-time.After(50 * time.Millisecond):
					break registerLoop
				}
			}

			require.Equal(t, tc.expectedCallbacks, callbacks)
		})
	}
}
