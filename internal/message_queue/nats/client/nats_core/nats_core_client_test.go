package nats_core_test

import (
	"errors"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
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

		expectedErrorStr     string
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
			publishErr: errors.New("failed to publish"),

			expectedErrorStr:     "failed to publish",
			expectedPublishCalls: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			natsMock := &mocks.NatsConnectionMock{
				PublishFunc: func(subj string, data []byte) error {
					return tc.publishErr
				},
			}
			logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			mqClient := nats_core.New(natsMock, nats_core.WithLogger(logger))

			err := mqClient.PublishMarshal(MinedTxsTopic, tc.txsBlock)

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
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

		expectedErrorStr     string
		expectedPublishCalls int
	}{
		{
			name: "success",

			expectedPublishCalls: 1,
		},
		{
			name:       "error - publish",
			publishErr: errors.New("failed to publish"),

			expectedErrorStr:     "failed to publish on register-tx topic",
			expectedPublishCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			natsMock := &mocks.NatsConnectionMock{
				PublishFunc: func(subj string, data []byte) error {
					return tc.publishErr
				},
			}

			mqClient := nats_core.New(
				natsMock,
			)

			err := mqClient.Publish(RegisterTxTopic, []byte("tx"))

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
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

		expectedErrorStr            string
		expectedQueueSubscribeCalls int
	}{
		{
			name: "success",

			expectedQueueSubscribeCalls: 1,
		},
		{
			name:         "error - publish",
			subscribeErr: errors.New("failed to subscribe"),

			expectedErrorStr:            "failed to subscribe to register-tx topic",
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
			var msgHandler nats.MsgHandler

			natsMock := &mocks.NatsConnectionMock{
				QueueSubscribeFunc: func(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
					require.Equal(t, "register-tx", subj)
					require.Equal(t, "register-tx-group", queue)
					msgHandler = cb
					return nil, tc.subscribeErr
				},
			}

			mqClient := nats_core.New(
				natsMock,
			)

			err := mqClient.Subscribe(RegisterTxTopic, func(bytes []byte) error { return tc.msgFuncErr })

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
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
		t.Run(tc.name, func(t *testing.T) {
			natsMock := &mocks.NatsConnectionMock{
				DrainFunc: func() error {
					return tc.drainErr
				},
			}

			mqClient := nats_core.New(
				natsMock,
			)

			mqClient.Shutdown()
		})
	}
}
