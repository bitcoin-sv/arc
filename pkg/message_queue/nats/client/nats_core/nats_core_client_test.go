package nats_core_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_core"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_core/mocks"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/test_api"
)

const (
	topic1 = "topic-1"
	topic2 = "topic-2"
)

func TestPublishMarshal(t *testing.T) {
	message := &test_api.TestMessage{
		Ok: true,
	}

	tt := []struct {
		name       string
		txsBlock   *test_api.TestMessage
		publishErr error

		expectedError        error
		expectedPublishCalls int
	}{
		{
			name:     "success",
			txsBlock: message,

			expectedPublishCalls: 1,
		},
		{
			name:       "publish err",
			txsBlock:   message,
			publishErr: nats_core.ErrFailedToPublish,

			expectedError:        nats_core.ErrFailedToPublish,
			expectedPublishCalls: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			natsMock := &mocks.NatsConnectionMock{
				PublishFunc: func(_ string, _ []byte) error {
					return tc.publishErr
				},
			}
			logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			sut := nats_core.New(natsMock, nats_core.WithLogger(logger))

			// when
			err := sut.PublishMarshal(context.TODO(), topic2, tc.txsBlock)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedPublishCalls, len(natsMock.PublishCalls()))
		})
	}
}

func TestPublish(t *testing.T) {
	tt := []struct {
		name       string
		publishErr error

		expectedError        error
		expectedPublishCalls int
	}{
		{
			name: "success",

			expectedPublishCalls: 1,
		},
		{
			name:       "error - publish",
			publishErr: nats_core.ErrFailedToPublish,

			expectedError:        nats_core.ErrFailedToPublish,
			expectedPublishCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			natsMock := &mocks.NatsConnectionMock{
				PublishFunc: func(_ string, _ []byte) error {
					return tc.publishErr
				},
			}

			sut := nats_core.New(
				natsMock,
			)

			// when
			err := sut.Publish(context.TODO(), topic1, []byte("tx"))

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			require.Equal(t, tc.expectedPublishCalls, len(natsMock.PublishCalls()))
		})
	}
}

func TestSubscribe(t *testing.T) {
	tt := []struct {
		name         string
		subscribeErr error
		msgFuncErr   error
		runFunc      bool

		expectedError               error
		expectedQueueSubscribeCalls int
	}{
		{
			name: "success",

			expectedQueueSubscribeCalls: 1,
		},
		{
			name:         "error - publish",
			subscribeErr: nats_core.ErrFailedToSubscribe,

			expectedError:               nats_core.ErrFailedToSubscribe,
			expectedQueueSubscribeCalls: 1,
		},
		{
			name:       "error - msg function",
			msgFuncErr: errors.New("function failed"),
			runFunc:    true,

			expectedQueueSubscribeCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var msgHandler nats.MsgHandler

			natsMock := &mocks.NatsConnectionMock{
				QueueSubscribeFunc: func(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
					require.Equal(t, "topic-1", subj)
					require.Equal(t, "topic-1-group", queue)
					msgHandler = cb
					return nil, tc.subscribeErr
				},
			}

			sut := nats_core.New(
				natsMock,
			)

			// when
			err := sut.Subscribe(topic1, func(_ []byte) error { return tc.msgFuncErr })

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			if tc.runFunc {
				msgHandler(&nats.Msg{})
			}

			require.Equal(t, tc.expectedQueueSubscribeCalls, len(natsMock.QueueSubscribeCalls()))
		})
	}
}

func TestShutdown(t *testing.T) {
	tt := []struct {
		name     string
		drainErr error
	}{
		{
			name:     "success",
			drainErr: nil,
		},
		{
			name:     "error - drain",
			drainErr: errors.New("failed to drain"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(_ *testing.T) {
			// given
			natsMock := &mocks.NatsConnectionMock{
				DrainFunc: func() error {
					return tc.drainErr
				},
			}

			// when
			sut := nats_core.New(
				natsMock,
			)

			// then
			sut.Shutdown()
		})
	}
}
