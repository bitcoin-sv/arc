package metamorph

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/mq"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPublishAdapter_StartPublishMarshal(t *testing.T) {
	txsBlocks := &blocktx_api.TransactionBlocks{
		TransactionBlocks: []*blocktx_api.TransactionBlock{
			{
				BlockHash:       []byte("1234"),
				BlockHeight:     10000,
				TransactionHash: []byte("1234"),
				MerklePath:      "1234",
				BlockStatus:     blocktx_api.Status_LONGEST,
				Timestamp:       timestamppb.New(time.Date(2025, 1, 1, 1, 1, 1, 0, time.UTC)),
			},
		},
	}
	dataMinedTx, err := proto.Marshal(txsBlocks)
	require.NoError(t, err)

	postRequest := &metamorph_api.PostTransactionRequest{
		CallbackUrl:   "https://abc.om",
		CallbackToken: "abc",
		CallbackBatch: false,
		RawTx:         nil,
	}
	submittedTx, err := proto.Marshal(postRequest)
	require.NoError(t, err)

	wrongMsg := []byte("wrong msg")

	tt := []struct {
		name                     string
		subscribeMinedTxsErr     error
		subscribeSubmittedTxsErr error
		consumeErr               error
		dataMinedTx              []byte
		dataSubmittedTx          []byte

		expectedError        error
		expectedMinedTxs     int
		expectedSubmittedTxs int
	}{
		{
			name:            "success",
			dataMinedTx:     dataMinedTx,
			dataSubmittedTx: submittedTx,

			expectedMinedTxs:     5,
			expectedSubmittedTxs: 5,
		},
		{
			name:            "no data",
			dataMinedTx:     nil,
			dataSubmittedTx: nil,

			expectedMinedTxs:     0,
			expectedSubmittedTxs: 0,
		},
		{
			name:            "consume error",
			dataMinedTx:     dataMinedTx,
			dataSubmittedTx: submittedTx,
			consumeErr:      errors.New("consume error"),

			expectedMinedTxs:     5,
			expectedSubmittedTxs: 5,
		},
		{
			name:            "consume error, wrong data",
			dataMinedTx:     dataMinedTx,
			dataSubmittedTx: wrongMsg,
			consumeErr:      errors.New("consume error"),

			expectedMinedTxs:     5,
			expectedSubmittedTxs: 0,
		},
		{
			name:                 "subscribe mined tx error",
			dataMinedTx:          dataMinedTx,
			dataSubmittedTx:      submittedTx,
			subscribeMinedTxsErr: errors.New("subscribe error"),
			expectedError:        ErrFailedToSubscribe,

			expectedMinedTxs:     0,
			expectedSubmittedTxs: 0,
		},
		{
			name:                     "subscribe submitted tx error",
			dataMinedTx:              dataMinedTx,
			dataSubmittedTx:          submittedTx,
			consumeErr:               errors.New("consume error"),
			subscribeSubmittedTxsErr: errors.New("subscribe error"),
			expectedError:            ErrFailedToSubscribe,

			expectedMinedTxs:     5,
			expectedSubmittedTxs: 0,
		},
		{
			name:            "wrong data",
			dataMinedTx:     wrongMsg,
			dataSubmittedTx: wrongMsg,

			expectedMinedTxs:     0,
			expectedSubmittedTxs: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create mock MQ client
			mqClient := &mqMocks.MessageQueueClientMock{
				QueueSubscribeFunc: func(topic string, msgFunc func([]byte) error) error {
					switch topic {
					case mq.MinedTxsTopic:
						if tc.subscribeMinedTxsErr != nil {
							return tc.subscribeMinedTxsErr
						}
						for range 5 {
							_ = msgFunc(tc.dataMinedTx)
						}
					case mq.SubmitTxTopic:
						if tc.subscribeSubmittedTxsErr != nil {
							return tc.subscribeSubmittedTxsErr
						}
						for range 5 {
							_ = msgFunc(tc.dataSubmittedTx)
						}
					default:
						t.Fatal("unexpected topic")
					}
					return nil
				},
				ConsumeFunc: func(_ string, msgFunc func([]byte) error) error {
					if tc.consumeErr != nil {
						return tc.consumeErr
					}
					for range 5 {
						_ = msgFunc(tc.dataSubmittedTx)
					}
					return nil
				},
			}

			logger := slog.Default()

			adapter := NewMessageSubscribeAdapter(mqClient, logger)
			defer adapter.Shutdown()

			minedTxsChan := make(chan *blocktx_api.TransactionBlocks, 10)
			submittedTxsChan := make(chan *metamorph_api.PostTransactionRequest, 10)

			actualError := adapter.Start(minedTxsChan, submittedTxsChan)

			minedTxs := 0
			submittedTxs := 0
		registerLoop:
			for {
				select {
				case <-minedTxsChan:
					minedTxs++
				case <-submittedTxsChan:
					submittedTxs++
				case <-time.After(50 * time.Millisecond):
					break registerLoop
				}
			}

			// Shutdown the adapter
			assert.Equal(t, tc.expectedMinedTxs, minedTxs)
			assert.Equal(t, tc.expectedSubmittedTxs, submittedTxs)

			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				return
			}
			require.NoError(t, actualError)
		})
	}
}
