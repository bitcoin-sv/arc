package metamorph_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"

	apiMocks "github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/bitcoin-sv/arc/pkg/metamorph/mocks"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestClient_SetUnlockedByName(t *testing.T) {
	tt := []struct {
		name           string
		setUnlockedErr error

		expectedErrorStr string
	}{
		{
			name: "success",
		},
		{
			name:           "err",
			setUnlockedErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := &apiMocks.MetaMorphAPIClientMock{
				SetUnlockedByNameFunc: func(ctx context.Context, in *metamorph_api.SetUnlockedByNameRequest, opts ...grpc.CallOption) (*metamorph_api.SetUnlockedByNameResponse, error) {
					return &metamorph_api.SetUnlockedByNameResponse{RecordsAffected: 5}, tc.setUnlockedErr
				},
			}

			client := metamorph.NewClient(apiClient)

			res, err := client.SetUnlockedByName(context.Background(), "test-1")
			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.Equal(t, int64(5), res)
		})
	}
}

func TestClient_SubmitTransaction(t *testing.T) {
	now := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	tt := []struct {
		name               string
		options            *metamorph.TransactionOptions
		putTxErr           error
		putTxStatus        *metamorph_api.TransactionStatus
		withMqClient       bool
		publishSubmitTxErr error

		expectedErrorStr string
		expectedStatus   *metamorph.TransactionStatus
	}{
		{
			name: "wait for received",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
			putTxStatus: &metamorph_api.TransactionStatus{
				Txid:   testdata.TX1Hash.String(),
				Status: metamorph_api.Status_RECEIVED,
			},

			expectedStatus: &metamorph.TransactionStatus{
				TxID:      testdata.TX1Hash.String(),
				Status:    metamorph_api.Status_RECEIVED.String(),
				Timestamp: now.Unix(),
			},
		},
		{
			name: "wait for seen - double spend attempted",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			putTxStatus: &metamorph_api.TransactionStatus{
				Txid:         testdata.TX1Hash.String(),
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"1234"},
			},

			expectedStatus: &metamorph.TransactionStatus{
				TxID:         testdata.TX1Hash.String(),
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED.String(),
				Timestamp:    now.Unix(),
				CompetingTxs: []string{"1234"},
			},
		},
		{
			name: "wait for received, put tx err, no mq client",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
			putTxStatus: &metamorph_api.TransactionStatus{
				Txid:   testdata.TX1Hash.String(),
				Status: metamorph_api.Status_RECEIVED,
			},
			putTxErr:     errors.New("failed to put tx"),
			withMqClient: false,

			expectedStatus:   nil,
			expectedErrorStr: "failed to put tx",
		},
		{
			name: "wait for queued, with mq client",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_QUEUED,
			},
			withMqClient: true,

			expectedStatus: &metamorph.TransactionStatus{
				TxID:      testdata.TX1Hash.String(),
				Status:    metamorph_api.Status_QUEUED.String(),
				Timestamp: now.Unix(),
			},
		},
		{
			name: "wait for queued, with mq client, publish submit tx err",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_QUEUED,
			},
			withMqClient:       true,
			publishSubmitTxErr: errors.New("failed to publish tx"),

			expectedStatus:   nil,
			expectedErrorStr: "failed to publish tx",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := &apiMocks.MetaMorphAPIClientMock{
				PutTransactionFunc: func(ctx context.Context, in *metamorph_api.TransactionRequest, opts ...grpc.CallOption) (*metamorph_api.TransactionStatus, error) {
					return tc.putTxStatus, tc.putTxErr
				},
			}

			opts := []func(client *metamorph.Metamorph){metamorph.WithNow(func() time.Time { return now })}
			if tc.withMqClient {
				mqClient := &mocks.MessageQueueClientMock{
					PublishMarshalFunc: func(topic string, m protoreflect.ProtoMessage) error { return tc.publishSubmitTxErr },
				}
				opts = append(opts, metamorph.WithMqClient(mqClient))
			}

			client := metamorph.NewClient(apiClient, opts...)

			tx, err := sdkTx.NewTransactionFromHex(testdata.TX1RawString)
			require.NoError(t, err)
			status, err := client.SubmitTransaction(context.Background(), tx, tc.options)

			require.Equal(t, tc.expectedStatus, status)

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}
		})
	}
}

func TestClient_SubmitTransactions(t *testing.T) {
	now := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	tx1, err := sdkTx.NewTransactionFromHex("010000000000000000ef016c50da4e8941c9b11720a4a29b40955c30f246b25740cd1aecffa2e3c4acd144000000006b483045022100eaf7791ec8ec1b9766473e70a5e41ac1734b6e43126d3dfa142c5f7670256cae02206169a3d22f0519b2631e8b952d8530db3502f2494ba038672e69b23a1e03340c412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffbf070000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01be070000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
	require.NoError(t, err)
	tx2, err := sdkTx.NewTransactionFromHex("010000000000000000ef0159f09a1fc4f1df5790730de57f96840fc5fbbbb08ebff52c986fd43a842588e0000000006a473044022020152c7c9f09e6b31bce86fc2b21bf8b0e5edfdaba575196dc47b933b4ec6f9502201815515de957ff44d8f9a9368a055d04bc7e1a675ad8b34ca67e47b28459718f412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffbf070000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01be070000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
	require.NoError(t, err)
	tx3, err := sdkTx.NewTransactionFromHex("010000000000000000ef016a4c158eb2906c84b3d95206a4dac765baf4dff63120e09ab0134dc6505a23bf000000006b483045022100a46fb3431796212efc3f78b2a8559ba66a5f197977ee983b765a8a1497c0e31a022077ff0eed59beadbdd08b9806ed1f83e41e57ef4892564a75ecbeddd99f14f60f412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffbf070000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01be070000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
	require.NoError(t, err)
	tt := []struct {
		name               string
		options            *metamorph.TransactionOptions
		putTxErr           error
		putTxStatus        *metamorph_api.TransactionStatuses
		withMqClient       bool
		publishSubmitTxErr error

		expectedErrorStr string
		expectedStatuses []*metamorph.TransactionStatus
	}{
		{
			name: "wait for received",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
			putTxStatus: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:   tx1.TxID(),
						Status: metamorph_api.Status_RECEIVED,
					},
					{
						Txid:   tx2.TxID(),
						Status: metamorph_api.Status_RECEIVED,
					},
					{
						Txid:   tx3.TxID(),
						Status: metamorph_api.Status_RECEIVED,
					},
				},
			},

			expectedStatuses: []*metamorph.TransactionStatus{
				{
					TxID:      tx1.TxID(),
					Status:    metamorph_api.Status_RECEIVED.String(),
					Timestamp: now.Unix(),
				},
				{
					TxID:      tx2.TxID(),
					Status:    metamorph_api.Status_RECEIVED.String(),
					Timestamp: now.Unix(),
				},
				{
					TxID:      tx3.TxID(),
					Status:    metamorph_api.Status_RECEIVED.String(),
					Timestamp: now.Unix(),
				},
			},
		},
		{
			name: "wait for received, put tx err, no mq client",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
			putTxStatus: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:   tx1.TxID(),
						Status: metamorph_api.Status_RECEIVED,
					},
					{
						Txid:   tx2.TxID(),
						Status: metamorph_api.Status_RECEIVED,
					},
					{
						Txid:   tx3.TxID(),
						Status: metamorph_api.Status_RECEIVED,
					},
				},
			},
			putTxErr:     errors.New("failed to put tx"),
			withMqClient: false,

			expectedStatuses: nil,
			expectedErrorStr: "failed to put tx",
		},
		{
			name: "wait for queued, with mq client",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_QUEUED,
			},
			withMqClient: true,

			expectedStatuses: []*metamorph.TransactionStatus{
				{
					TxID:      tx1.TxID(),
					Status:    metamorph_api.Status_QUEUED.String(),
					Timestamp: now.Unix(),
				},
				{
					TxID:      tx2.TxID(),
					Status:    metamorph_api.Status_QUEUED.String(),
					Timestamp: now.Unix(),
				},
				{
					TxID:      tx3.TxID(),
					Status:    metamorph_api.Status_QUEUED.String(),
					Timestamp: now.Unix(),
				},
			},
		},
		{
			name: "wait for queued, with mq client, publish submit tx err",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_QUEUED,
			},
			withMqClient:       true,
			publishSubmitTxErr: errors.New("failed to publish tx"),

			expectedStatuses: nil,
			expectedErrorStr: "failed to publish tx",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := &apiMocks.MetaMorphAPIClientMock{
				PutTransactionsFunc: func(ctx context.Context, in *metamorph_api.TransactionRequests, opts ...grpc.CallOption) (*metamorph_api.TransactionStatuses, error) {
					return tc.putTxStatus, tc.putTxErr
				},
			}

			opts := []func(client *metamorph.Metamorph){metamorph.WithNow(func() time.Time { return now })}
			if tc.withMqClient {
				mqClient := &mocks.MessageQueueClientMock{
					PublishMarshalFunc: func(topic string, m protoreflect.ProtoMessage) error {
						return tc.publishSubmitTxErr
					},
				}
				opts = append(opts, metamorph.WithMqClient(mqClient))
			}

			client := metamorph.NewClient(apiClient, opts...)

			statuses, err := client.SubmitTransactions(context.Background(), sdkTx.Transactions{tx1, tx2, tx3}, tc.options)

			require.Equal(t, tc.expectedStatuses, statuses)

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}
		})
	}
}

func TestTransactionHandler_GetTransaction(t *testing.T) {
	ctx := context.Background()
	txID := "testTxID"

	mockHandler := &mocks.TransactionHandlerMock{
		GetTransactionFunc: func(ctx context.Context, txID string) ([]byte, error) {
			if txID == "testTxID" {
				return []byte("testdata"), nil
			}
			return nil, metamorph.ErrTransactionNotFound
		},
	}

	// Test case: Transaction found
	txData, err := mockHandler.GetTransaction(ctx, txID)
	require.NoError(t, err)
	require.Equal(t, []byte("testdata"), txData)

	// Test case: Transaction not found
	_, err = mockHandler.GetTransaction(ctx, "invalidTxID")
	require.ErrorIs(t, err, metamorph.ErrTransactionNotFound)

	// Ensure GetTransaction was called twice
	require.Equal(t, 2, len(mockHandler.GetTransactionCalls()))
}

func TestTransactionHandler_Health(t *testing.T) {
	ctx := context.Background()

	mockHandler := &mocks.TransactionHandlerMock{
		HealthFunc: func(ctx context.Context) error {
			return nil
		},
	}

	// Test case: Health check success
	err := mockHandler.Health(ctx)
	require.NoError(t, err)

	// Ensure Health was called once
	require.Equal(t, 1, len(mockHandler.HealthCalls()))
}

func TestTransactionHandler_SubmitTransactionWithError(t *testing.T) {
	ctx := context.Background()
	tx := &sdkTx.Transaction{}
	options := &metamorph.TransactionOptions{
		CallbackURL: "http://example.com/callback",
	}

	mockHandler := &mocks.TransactionHandlerMock{
		SubmitTransactionFunc: func(ctx context.Context, tx *sdkTx.Transaction, options *metamorph.TransactionOptions) (*metamorph.TransactionStatus, error) {
			return nil, errors.New("failed to submit transaction")
		},
	}

	// Test case: Submit transaction with an error
	status, err := mockHandler.SubmitTransaction(ctx, tx, options)
	require.Error(t, err)
	require.Nil(t, status)
	require.EqualError(t, err, "failed to submit transaction")

	// Ensure SubmitTransaction was called once
	require.Equal(t, 1, len(mockHandler.SubmitTransactionCalls()))
}

func TestTransactionHandler_SubmitEmptyTransaction(t *testing.T) {
	ctx := context.Background()
	options := &metamorph.TransactionOptions{
		CallbackURL: "http://example.com/callback",
	}

	mockHandler := &mocks.TransactionHandlerMock{
		SubmitTransactionFunc: func(ctx context.Context, tx *sdkTx.Transaction, options *metamorph.TransactionOptions) (*metamorph.TransactionStatus, error) {
			if tx == nil {
				return nil, errors.New("transaction cannot be nil")
			}
			return &metamorph.TransactionStatus{
				TxID:   "nilTx",
				Status: "ACCEPTED",
			}, nil
		},
	}

	// Test case: Submit nil transaction
	status, err := mockHandler.SubmitTransaction(ctx, nil, options)
	require.Error(t, err)
	require.Nil(t, status)
	require.EqualError(t, err, "transaction cannot be nil")

	// Ensure SubmitTransaction was called once
	require.Equal(t, 1, len(mockHandler.SubmitTransactionCalls()))
}

func TestTransactionHandler_SubmitLargeNumberOfTransactions(t *testing.T) {
	ctx := context.Background()
	txs := make(sdkTx.Transactions, 1000) // Simulating 1000 transactions
	for i := range txs {
		txs[i] = &sdkTx.Transaction{}
	}
	options := &metamorph.TransactionOptions{
		CallbackURL: "http://example.com/callback",
	}

	mockHandler := &mocks.TransactionHandlerMock{
		SubmitTransactionsFunc: func(ctx context.Context, txs sdkTx.Transactions, options *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
			statuses := make([]*metamorph.TransactionStatus, len(txs))
			for i := range txs {
				statuses[i] = &metamorph.TransactionStatus{
					TxID:   "txID" + fmt.Sprint(i),
					Status: "ACCEPTED",
				}
			}
			return statuses, nil
		},
	}

	// Test case: Submit a large number of transactions
	statuses, err := mockHandler.SubmitTransactions(ctx, txs, options)
	require.NoError(t, err)
	require.Len(t, statuses, 1000)

	// Ensure SubmitTransactions was called once
	require.Equal(t, 1, len(mockHandler.SubmitTransactionsCalls()))
}

func TestTransactionHandler_GetTransactionsWithEmptyList(t *testing.T) {
	ctx := context.Background()

	mockHandler := &mocks.TransactionHandlerMock{
		GetTransactionsFunc: func(ctx context.Context, txIDs []string) ([]*metamorph.Transaction, error) {
			if len(txIDs) == 0 {
				return nil, errors.New("transaction IDs cannot be empty")
			}
			return []*metamorph.Transaction{}, nil
		},
	}

	// Test case: Get transactions with an empty list of transaction IDs
	txs, err := mockHandler.GetTransactions(ctx, []string{})
	require.Error(t, err)
	require.Nil(t, txs)
	require.EqualError(t, err, "transaction IDs cannot be empty")

	// Ensure GetTransactions was called once
	require.Equal(t, 1, len(mockHandler.GetTransactionsCalls()))
}

func TestTransactionHandler_HealthWithError(t *testing.T) {
	ctx := context.Background()

	mockHandler := &mocks.TransactionHandlerMock{
		HealthFunc: func(ctx context.Context) error {
			return errors.New("health check failed")
		},
	}

	// Test case: Health check returns an error
	err := mockHandler.Health(ctx)
	require.Error(t, err)
	require.EqualError(t, err, "health check failed")

	// Ensure Health was called once
	require.Equal(t, 1, len(mockHandler.HealthCalls()))
}
