package natscore_test

import (
	"errors"
	natscore "github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core/mocks"
	"log/slog"
	"os"
	"testing"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

const (
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
)

func TestPublishMarshal(t *testing.T) {
	txBlock := &blocktx_api.TransactionBlock{
		BlockHash:       testdata.Block1Hash[:],
		BlockHeight:     1,
		TransactionHash: testdata.TX1Hash[:],
		MerklePath:      "mp-1",
	}

	tt := []struct {
		name       string
		txsBlock   *blocktx_api.TransactionBlock
		publishErr error

		expectedError        error
		expectedPublishCalls int
	}{
		{
			name:     "success",
			txsBlock: txBlock,

			expectedPublishCalls: 1,
		},
		{
			name:       "publish err",
			txsBlock:   txBlock,
			publishErr: natscore.ErrFailedToPublish,

			expectedError:        natscore.ErrFailedToPublish,
			expectedPublishCalls: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			//given
			natsMock := &mocks.NatsConnectionMock{
				PublishFunc: func(_ string, _ []byte) error {
					return tc.publishErr
				},
			}
			logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			sut := natscore.New(natsMock, natscore.WithLogger(logger))

			// when
			err := sut.PublishMarshal(MinedTxsTopic, tc.txsBlock)

			// then
			if tc.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

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
			publishErr: natscore.ErrFailedToPublish,

			expectedError:        natscore.ErrFailedToPublish,
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

			sut := natscore.New(
				natsMock,
			)

			// when
			err := sut.Publish(RegisterTxTopic, []byte("tx"))

			// then
			if tc.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

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
			subscribeErr: natscore.ErrFailedToSubscribe,

			expectedError:               natscore.ErrFailedToSubscribe,
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
					require.Equal(t, "register-tx", subj)
					require.Equal(t, "register-tx-group", queue)
					msgHandler = cb
					return nil, tc.subscribeErr
				},
			}

			sut := natscore.New(
				natsMock,
			)

			// when
			err := sut.Subscribe(RegisterTxTopic, func(_ []byte) error { return tc.msgFuncErr })

			// then
			if tc.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

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
			sut := natscore.New(
				natsMock,
			)

			// then
			sut.Shutdown()
		})
	}
}
