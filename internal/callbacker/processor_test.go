package callbacker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

func TestProcessorStart(t *testing.T) {
	request := &callbacker_api.SendRequest{
		CallbackRouting: &callbacker_api.CallbackRouting{
			Url:        "https://callbacks.com",
			Token:      "1234",
			AllowBatch: false,
		},
		Txid:   "e045bac7e7b3328dc4c986dafcffc1570830683518fd8fe8d6e94ff545d9ef5c",
		Status: callbacker_api.Status_SEEN_ON_NETWORK,
	}
	data, err := proto.Marshal(request)
	require.NoError(t, err)

	tt := []struct {
		name             string
		setURLMappingErr error
		mappings         map[string]string
		messageData      []byte
		nakErr           error
		ackErr           error

		expectedError              error
		expectedMessageAckCalls    int
		expectedMessageNakCalls    int
		expectedSetURLMappingCalls int
		expectedDispatchCalls      int
	}{
		{
			name:        "URL found & matching",
			mappings:    map[string]string{"https://callbacks.com": "host1"},
			messageData: data,

			expectedMessageAckCalls:    1,
			expectedMessageNakCalls:    0,
			expectedSetURLMappingCalls: 0,
			expectedDispatchCalls:      1,
		},
		{
			name:        "URL found & matching - ack err",
			mappings:    map[string]string{"https://callbacks.com": "host1"},
			messageData: data,
			ackErr:      errors.New("ack failed"),

			expectedMessageAckCalls:    1,
			expectedMessageNakCalls:    0,
			expectedSetURLMappingCalls: 0,
			expectedDispatchCalls:      1,
		},
		{
			name:        "URL found & not matching",
			mappings:    map[string]string{"https://callbacks.com": "host2"},
			messageData: data,

			expectedMessageAckCalls:    1,
			expectedMessageNakCalls:    0,
			expectedSetURLMappingCalls: 0,
			expectedDispatchCalls:      0,
		},
		{
			name:        "invalid data",
			mappings:    map[string]string{"https://callbacks.com": "host1"},
			messageData: []byte("invalid data"),

			expectedError:              callbacker.ErrUnmarshal,
			expectedMessageAckCalls:    0,
			expectedMessageNakCalls:    1,
			expectedSetURLMappingCalls: 0,
			expectedDispatchCalls:      0,
		},
		{
			name:        "invalid data - nak failed",
			mappings:    map[string]string{"https://callbacks.com": "host1"},
			messageData: []byte("invalid data"),
			nakErr:      errors.New("nak failed"),

			expectedError:              callbacker.ErrUnmarshal,
			expectedMessageAckCalls:    0,
			expectedMessageNakCalls:    1,
			expectedSetURLMappingCalls: 0,
			expectedDispatchCalls:      0,
		},
		{
			name:        "URL not found - succeeded to set mapping",
			mappings:    map[string]string{"https://callbacks-2.com": "host2"},
			messageData: data,

			expectedMessageAckCalls:    1,
			expectedMessageNakCalls:    0,
			expectedSetURLMappingCalls: 1,
			expectedDispatchCalls:      1,
		},
		{
			name:             "URL not found - failed to set URL mapping - duplicate key",
			mappings:         map[string]string{"https://callbacks-2.com": "host2"},
			setURLMappingErr: store.ErrURLMappingDuplicateKey,
			messageData:      data,

			expectedMessageAckCalls:    1,
			expectedMessageNakCalls:    0,
			expectedSetURLMappingCalls: 1,
			expectedDispatchCalls:      0,
		},
		{
			name:             "URL not found - failed to set URL mapping - duplicate key - ack failed",
			mappings:         map[string]string{"https://callbacks-2.com": "host2"},
			setURLMappingErr: store.ErrURLMappingDuplicateKey,
			messageData:      data,
			ackErr:           errors.New("ack failed"),

			expectedMessageAckCalls:    1,
			expectedMessageNakCalls:    0,
			expectedSetURLMappingCalls: 1,
			expectedDispatchCalls:      0,
		},
		{
			name:             "URL not found - failed to set URL mapping - other error",
			mappings:         map[string]string{"https://callbacks-2.com": "host2"},
			setURLMappingErr: errors.New("failed to set URL mapping"),
			messageData:      data,

			expectedError:              callbacker.ErrSetMapping,
			expectedMessageAckCalls:    0,
			expectedMessageNakCalls:    1,
			expectedSetURLMappingCalls: 1,
			expectedDispatchCalls:      0,
		},
		{
			name:             "URL not found - failed to set URL mapping - other error - nak failed",
			mappings:         map[string]string{"https://callbacks-2.com": "host2"},
			setURLMappingErr: errors.New("failed to set URL mapping"),
			nakErr:           errors.New("nak failed"),
			messageData:      data,

			expectedError:              callbacker.ErrSetMapping,
			expectedMessageAckCalls:    0,
			expectedMessageNakCalls:    1,
			expectedSetURLMappingCalls: 1,
			expectedDispatchCalls:      0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher := &mocks.DispatcherMock{
				DispatchFunc: func(_ string, _ *callbacker.CallbackEntry, _ bool) {},
			}
			processorStore := &mocks.ProcessorStoreMock{
				SetURLMappingFunc:    func(_ context.Context, _ store.URLMapping) error { return tc.setURLMappingErr },
				DeleteURLMappingFunc: func(_ context.Context, _ string) error { return nil },
				GetURLMappingsFunc: func(_ context.Context) (map[string]string, error) {
					return tc.mappings, nil
				},
			}

			var handleMsgFunction func(msg jetstream.Msg) error
			mqClient := &mocks.MessageQueueClientMock{
				SubscribeMsgFunc: func(_ string, msgFunc func(msg jetstream.Msg) error) error {
					handleMsgFunction = msgFunc
					return nil
				},
				ShutdownFunc: func() {},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			processor, err := callbacker.NewProcessor(dispatcher, processorStore, mqClient, "host1", logger)
			require.NoError(t, err)

			err = processor.Start()
			require.NoError(t, err)

			message := &mocks.JetstreamMsgMock{
				DataFunc: func() []byte {
					return tc.messageData
				},
				NakFunc: func() error {
					return tc.nakErr
				},
				AckFunc: func() error {
					return tc.ackErr
				},
			}

			time.Sleep(100 * time.Millisecond)

			actualErr := handleMsgFunction(message)
			if tc.expectedError != nil {
				require.ErrorIs(t, actualErr, tc.expectedError)
				return
			}
			require.NoError(t, actualErr)

			time.Sleep(100 * time.Millisecond)

			processor.GracefulStop()

			require.Equal(t, tc.expectedMessageAckCalls, len(message.AckCalls()))
			require.Equal(t, tc.expectedMessageNakCalls, len(message.NakCalls()))
			require.Equal(t, tc.expectedSetURLMappingCalls, len(processorStore.SetURLMappingCalls()))
			require.Equal(t, tc.expectedDispatchCalls, len(dispatcher.DispatchCalls()))
		})
	}
}
