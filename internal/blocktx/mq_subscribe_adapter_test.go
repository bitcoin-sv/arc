package blocktx

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/mq"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestPublishAdapter_StartPublishMarshal(t *testing.T) {
	blockTxs := &blocktx_api.Transactions{Transactions: []*blocktx_api.Transaction{{Hash: []byte("1234")}}}
	dataRegisterTxs, err := proto.Marshal(blockTxs)
	require.NoError(t, err)

	dataRegisterTx := []byte("1234")

	wrongMsg := []byte("wrong msg")

	tt := []struct {
		name                    string
		subscribeRegisterTxErr  error
		subscribeRegisterTxsErr error
		dataRegisterTx          []byte
		dataRegisterTxs         []byte

		expectedError         error
		expectedRegisteredTxs int
	}{
		{
			name:            "success",
			dataRegisterTx:  dataRegisterTx,
			dataRegisterTxs: dataRegisterTxs,

			expectedRegisteredTxs: 10,
		},
		{
			name:            "no data",
			dataRegisterTx:  nil,
			dataRegisterTxs: nil,

			expectedRegisteredTxs: 0,
		},
		{
			name:            "wrong data",
			dataRegisterTx:  dataRegisterTx,
			dataRegisterTxs: wrongMsg,

			expectedRegisteredTxs: 5,
		},
		{
			name:                   "error subscribe register tx",
			dataRegisterTx:         dataRegisterTx,
			dataRegisterTxs:        dataRegisterTxs,
			subscribeRegisterTxErr: errors.New("subscribe error"),

			expectedError:         ErrFailedToSubscribeToTopic,
			expectedRegisteredTxs: 0,
		},
		{
			name:                    "error subscribe register txs",
			dataRegisterTx:          dataRegisterTx,
			dataRegisterTxs:         dataRegisterTxs,
			subscribeRegisterTxsErr: errors.New("subscribe error"),

			expectedError:         ErrFailedToSubscribeToTopic,
			expectedRegisteredTxs: 5,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create mock MQ client
			mqClient := &mqMocks.MessageQueueClientMock{
				QueueSubscribeFunc: func(topic string, msgFunc func([]byte) error) error {
					switch topic {
					case mq.RegisterTxTopic:
						if tc.subscribeRegisterTxErr != nil {
							return tc.subscribeRegisterTxErr
						}
						for range 5 {
							_ = msgFunc(tc.dataRegisterTx)
						}
					case mq.RegisterTxsTopic:
						if tc.subscribeRegisterTxsErr != nil {
							return tc.subscribeRegisterTxsErr
						}
						for range 5 {
							_ = msgFunc(tc.dataRegisterTxs)
						}
					default:
						t.Fatal("unexpected topic")
					}
					return nil
				},
			}

			logger := slog.Default()

			adapter := NewMessageSubscribeAdapter(mqClient, logger)
			defer adapter.Shutdown()

			registerTxChan := make(chan []byte, 10)

			actualError := adapter.Start(registerTxChan)

			registerTxs := 0
		registerLoop:
			for {
				select {
				case <-registerTxChan:
					registerTxs++
				case <-time.After(50 * time.Millisecond):
					break registerLoop
				}
			}

			// Shutdown the adapter
			assert.Equal(t, tc.expectedRegisteredTxs, registerTxs)

			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				return
			}
			require.NoError(t, actualError)
		})
	}
}
