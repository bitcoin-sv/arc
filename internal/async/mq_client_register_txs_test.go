package async_test

import (
	"errors"
	"testing"

	"github.com/bitcoin-sv/arc/internal/async"
	"github.com/bitcoin-sv/arc/internal/async/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestMQClient_PublishRegisterTxs(t *testing.T) {

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
				DrainFunc: func() error { return nil },
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

			err = mqClient.Shutdown()
			require.NoError(t, err)
		})
	}
}

func TestMQClient_SubscribeRegisterTxs(t *testing.T) {

	tt := []struct {
		name                string
		withRegisterTxsChan bool
		errQueueSubscribe   error

		expectedErrorStr string
	}{
		{
			name:                "success",
			withRegisterTxsChan: true,
		},
		{
			name:                "error - missing mined txs channel",
			withRegisterTxsChan: false,

			expectedErrorStr: "mined txs channel is nil",
		},
		{
			name:                "error - queue subscribe",
			withRegisterTxsChan: true,

			errQueueSubscribe: errors.New("failed to subscribe"),
			expectedErrorStr:  "failed to subscribe to mined-txs topic",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			natsMock := &mocks.NatsClientMock{
				QueueSubscribeFunc: func(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
					return nil, tc.errQueueSubscribe
				},
				DrainFunc: func() error { return nil },
			}

			opts := []func(*async.MQClient){
				async.WithMaxBatchSize(5),
			}

			if tc.withRegisterTxsChan {
				minedTxsChan := make(chan *blocktx_api.TransactionBlock, 5)
				opts = append(opts, async.WithMinedTxsChan(minedTxsChan))
			}

			mqClient := async.NewNatsMQClient(
				natsMock,
				opts...,
			)

			err := mqClient.SubscribeRegisterTxs()

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			err = mqClient.Shutdown()
			require.NoError(t, err)
		})
	}
}
