package callbacker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestProcessorStart(t *testing.T) {
	request := &callbacker_api.SendRequest{
		CallbackRouting: &callbacker_api.CallbackRouting{
			Url:        "https://callbacks.com",
			Token:      "1234",
			AllowBatch: false,
		},
		Txid:         "e045bac7e7b3328dc4c986dafcffc1570830683518fd8fe8d6e94ff545d9ef5c",
		BlockHash:    "0000000000000000102a010fea315fd594479fc4b1284288780c61a275ebc4f6",
		ExtraInfo:    "Transaction Info",
		BlockHeight:  782318,
		MerklePath:   "0000",
		CompetingTxs: []string{"tx1", "tx2"},
		Status:       callbacker_api.Status_SEEN_ON_NETWORK,
	}
	data, err := proto.Marshal(request)
	require.NoError(t, err)

	requestEmptyURL := &callbacker_api.SendRequest{
		CallbackRouting: &callbacker_api.CallbackRouting{
			Url: "",
		},
		Txid:   "e045bac7e7b3328dc4c986dafcffc1570830683518fd8fe8d6e94ff545d9ef5c",
		Status: callbacker_api.Status_SEEN_ON_NETWORK,
	}

	dataEmptyURL, err := proto.Marshal(requestEmptyURL)
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
			name:        "empty URL",
			mappings:    map[string]string{"https://callbacks.com": "host1"},
			messageData: dataEmptyURL,

			expectedMessageAckCalls:    1,
			expectedMessageNakCalls:    0,
			expectedSetURLMappingCalls: 0,
			expectedDispatchCalls:      0,
		},
		{
			name:        "URL found & matching - ack err",
			mappings:    map[string]string{"https://callbacks.com": "host1"},
			messageData: data,
			ackErr:      errors.New("ack failed"),

			expectedError:              callbacker.ErrAckMessage,
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

			expectedError:              callbacker.ErrAckMessage,
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
				DispatchFunc: func(_ string, _ *callbacker.CallbackEntry) {},
			}
			processorStore := &mocks.ProcessorStoreMock{
				SetURLMappingFunc: func(_ context.Context, _ store.URLMapping) error { return tc.setURLMappingErr },
				DeleteURLMappingFunc: func(_ context.Context, _ string) (int64, error) {
					return 0, nil
				},
				GetURLMappingsFunc: func(_ context.Context) (map[string]string, error) {
					return tc.mappings, nil
				},
			}

			var handleMsgFunction func(msg jetstream.Msg) error
			mqClient := &mqMocks.MessageQueueClientMock{
				ConsumeMsgFunc: func(_ string, msgFunc func(msg jetstream.Msg) error) error {
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

func TestDispatchPersistedCallbacks(t *testing.T) {
	tt := []struct {
		name              string
		storedCallbacks   []*store.CallbackData
		getAndDeleteErr   error
		setURLMappingErr  error
		getUnmappedURLErr error

		expectedDispatch           int
		expectedSetURLMappingCalls int
		expectedGetAndDeleteCalls  int
	}{
		{
			name:            "success - no stored callbacks",
			storedCallbacks: []*store.CallbackData{},

			expectedDispatch:           0,
			expectedSetURLMappingCalls: 1,
			expectedGetAndDeleteCalls:  1,
		},
		{
			name: "success - 1 stored callback",
			storedCallbacks: []*store.CallbackData{{
				URL:   "https://test.com",
				Token: "1234",
			}},

			expectedDispatch:           1,
			expectedSetURLMappingCalls: 1,
			expectedGetAndDeleteCalls:  1,
		},
		{
			name: "URL already mapped",
			storedCallbacks: []*store.CallbackData{{
				URL:   "https://test.com",
				Token: "1234",
			}},
			setURLMappingErr: store.ErrURLMappingDuplicateKey,

			expectedDispatch:           0,
			expectedSetURLMappingCalls: 1,
			expectedGetAndDeleteCalls:  0,
		},
		{
			name:            "error deleting failed older than",
			getAndDeleteErr: errors.New("some error"),

			expectedDispatch:           0,
			expectedSetURLMappingCalls: 1,
			expectedGetAndDeleteCalls:  1,
		},
		{
			name:              "error - no unmapped URL callbacks",
			getUnmappedURLErr: errors.New("failed to get unmapped URL callbacks"),

			expectedDispatch:           0,
			expectedSetURLMappingCalls: 0,
			expectedGetAndDeleteCalls:  0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.Default()
			dispatcher := &mocks.DispatcherMock{DispatchFunc: func(_ string, _ *callbacker.CallbackEntry) {}}

			mqClient := &mqMocks.MessageQueueClientMock{
				ConsumeMsgFunc: func(_ string, _ func(_ jetstream.Msg) error) error {
					return nil
				},
				ShutdownFunc: func() {},
			}
			processorStore := &mocks.ProcessorStoreMock{
				SetURLMappingFunc: func(_ context.Context, _ store.URLMapping) error { return tc.setURLMappingErr },
				DeleteURLMappingFunc: func(_ context.Context, _ string) (int64, error) {
					return 0, nil
				},
				GetURLMappingsFunc: func(_ context.Context) (map[string]string, error) { return nil, nil },
				GetUnmappedURLFunc: func(_ context.Context) (string, error) { return "https://abcdefg.com", tc.getUnmappedURLErr },
				GetAndDeleteFunc: func(_ context.Context, _ string, _ int) ([]*store.CallbackData, error) {
					return tc.storedCallbacks, tc.getAndDeleteErr
				},
			}
			processor, err := callbacker.NewProcessor(dispatcher, processorStore, mqClient, "host1", logger, callbacker.WithDispatchPersistedInterval(20*time.Millisecond))
			require.NoError(t, err)

			defer processor.GracefulStop()

			processor.StartSetUnmappedURLs()
			time.Sleep(30 * time.Millisecond)

			require.Equal(t, tc.expectedDispatch, len(dispatcher.DispatchCalls()))
			require.Equal(t, tc.expectedSetURLMappingCalls, len(processorStore.SetURLMappingCalls()))
			require.Equal(t, tc.expectedGetAndDeleteCalls, len(processorStore.GetAndDeleteCalls()))
		})
	}
}

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
				DeleteOlderThanFunc: func(_ context.Context, _ time.Time) error {
					return tc.deleteFailedOlderThanErr
				},
				DeleteURLMappingFunc: func(_ context.Context, _ string) (int64, error) {
					return 0, nil
				},
			}
			dispatcher := &mocks.DispatcherMock{}

			processor, err := callbacker.NewProcessor(dispatcher, cbStore, nil, "hostname", slog.Default())
			require.NoError(t, err)
			defer processor.GracefulStop()

			processor.StartCallbackStoreCleanup(20*time.Millisecond, 50*time.Second)

			time.Sleep(90 * time.Millisecond)

			require.Equal(t, tc.expectedIterations, len(cbStore.DeleteOlderThanCalls()))
		})
	}
}
