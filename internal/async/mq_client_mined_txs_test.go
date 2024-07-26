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

func TestMQClient_PublishMinedTxs(t *testing.T) {
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
				DrainFunc: func() error { return nil },
			}

			mqClient := async.NewNatsMQClient(
				natsMock,
				async.WithMaxBatchSize(5),
			)

			err := mqClient.PublishMarshal(async.MinedTxsTopic, tc.txsBlock)

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

func TestMQClient_SubscribeMinedTxs(t *testing.T) {

	tt := []struct {
		name              string
		withMinedTxsChan  bool
		errQueueSubscribe error

		expectedErrorStr string
	}{
		{
			name:             "success",
			withMinedTxsChan: true,
		},
		{
			name:             "error - missing mined txs channel",
			withMinedTxsChan: false,

			expectedErrorStr: "mined txs channel is nil",
		},
		{
			name:             "error - queue subscribe",
			withMinedTxsChan: true,

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

			if tc.withMinedTxsChan {
				minedTxsChan := make(chan *blocktx_api.TransactionBlock, 1)
				opts = append(opts, async.WithMinedTxsChan(minedTxsChan))
			}

			mqClient := async.NewNatsMQClient(
				natsMock,
				opts...,
			)

			err := mqClient.SubscribeMinedTxs()

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
