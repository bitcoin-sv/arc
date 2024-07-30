package async_test

import (
	"errors"
	"testing"

	"github.com/bitcoin-sv/arc/internal/async"
	"github.com/bitcoin-sv/arc/internal/async/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
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
			natsMock := &mocks.NatsClientMock{
				PublishFunc: func(subj string, data []byte) error {
					return tc.publishErr
				},
			}

			mqClient := async.NewNatsMQClient(natsMock)

			err := mqClient.PublishMarshal(async.MinedTxsTopic, tc.txsBlock)

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
			natsMock := &mocks.NatsClientMock{
				PublishFunc: func(subj string, data []byte) error {
					return tc.publishErr
				},
			}

			mqClient := async.NewNatsMQClient(
				natsMock,
			)

			err := mqClient.Publish(async.RegisterTxTopic, []byte("tx"))

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
		name       string
		publishErr error

		expectedErrorStr            string
		expectedQueueSubscribeCalls int
	}{
		{
			name: "success",

			expectedQueueSubscribeCalls: 1,
		},
		{
			name:       "error - publish",
			publishErr: errors.New("failed to publish"),

			expectedErrorStr:            "failed to subscribe to register-tx topic",
			expectedQueueSubscribeCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			natsMock := &mocks.NatsClientMock{
				QueueSubscribeFunc: func(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
					require.Equal(t, "register-tx", subj)
					require.Equal(t, "register-tx-group", queue)

					return nil, tc.publishErr
				},
			}

			mqClient := async.NewNatsMQClient(
				natsMock,
			)

			err := mqClient.Subscribe(async.RegisterTxTopic, func(msg *nats.Msg) {})

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
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
			natsMock := &mocks.NatsClientMock{
				DrainFunc: func() error {
					return tc.drainErr
				},
			}

			mqClient := async.NewNatsMQClient(
				natsMock,
			)

			mqClient.Shutdown()
		})
	}
}
