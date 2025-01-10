package metamorph_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"

	apiMocks "github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/emptypb"
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
			// Given
			apiClient := &apiMocks.MetaMorphAPIClientMock{
				SetUnlockedByNameFunc: func(_ context.Context, _ *metamorph_api.SetUnlockedByNameRequest, _ ...grpc.CallOption) (*metamorph_api.SetUnlockedByNameResponse, error) {
					return &metamorph_api.SetUnlockedByNameResponse{RecordsAffected: 5}, tc.setUnlockedErr
				},
			}

			client := metamorph.NewClient(apiClient)
			// When
			res, err := client.SetUnlockedByName(context.Background(), "test-1")
			// Then
			if tc.expectedErrorStr != "" {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}
			require.NoError(t, err)
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
		getTxErr           error
		getTxStatus        *metamorph_api.TransactionStatus
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
			name: "wait for received, put tx err",
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
			name: "wait for received, put tx err deadline exceeded",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
			putTxStatus: &metamorph_api.TransactionStatus{
				Txid:   testdata.TX1Hash.String(),
				Status: metamorph_api.Status_RECEIVED,
			},
			withMqClient: false,
			getTxStatus: &metamorph_api.TransactionStatus{
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
			name: "wait for received, put tx err deadline exceeded, get tx err",
			options: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
			putTxStatus: &metamorph_api.TransactionStatus{
				Txid:     testdata.TX1Hash.String(),
				Status:   metamorph_api.Status_RECEIVED,
				TimedOut: true,
			},
			withMqClient: false,
			getTxErr:     errors.New("failed to get tx status"),
			expectedStatus: &metamorph.TransactionStatus{
				TxID:      testdata.TX1Hash.String(),
				Status:    metamorph_api.Status_RECEIVED.String(),
				Timestamp: now.Unix(),
			},
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
			// Given
			apiClient := &apiMocks.MetaMorphAPIClientMock{
				PutTransactionFunc: func(_ context.Context, _ *metamorph_api.TransactionRequest, _ ...grpc.CallOption) (*metamorph_api.TransactionStatus, error) {
					return tc.putTxStatus, tc.putTxErr
				},
				GetTransactionStatusFunc: func(_ context.Context, _ *metamorph_api.TransactionStatusRequest, _ ...grpc.CallOption) (*metamorph_api.TransactionStatus, error) {
					return tc.getTxStatus, tc.getTxErr
				},
			}

			opts := []func(client *metamorph.Metamorph){
				metamorph.WithClientNow(func() time.Time { return now }),
			}
			if tc.withMqClient {
				mqClient := &apiMocks.MessageQueueClientMock{
					PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error { return tc.publishSubmitTxErr },
				}
				opts = append(opts, metamorph.WithMqClient(mqClient))
			}

			client := metamorph.NewClient(apiClient, opts...)
			// When
			tx, err := sdkTx.NewTransactionFromHex(testdata.TX1RawString)
			// Then
			require.NoError(t, err)
			ctx := context.Background()
			timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
			txStatus, err := client.SubmitTransaction(timeoutCtx, tx, tc.options)

			require.Equal(t, tc.expectedStatus, txStatus)

			if tc.expectedErrorStr != "" {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}
			require.NoError(t, err)
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
		getTxErr           error
		getTxStatus        *metamorph_api.TransactionStatus
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
			name: "wait for received, put tx err deadline exceeded",
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
			withMqClient: false,
			getTxStatus: &metamorph_api.TransactionStatus{
				Txid:   testdata.TX1Hash.String(),
				Status: metamorph_api.Status_RECEIVED,
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
			name: "wait for received, put tx err deadline exceeded, get tx err",
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
			withMqClient: false,
			getTxErr:     errors.New("failed to get tx status"),

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
			// Given
			apiClient := &apiMocks.MetaMorphAPIClientMock{
				PutTransactionsFunc: func(_ context.Context, _ *metamorph_api.TransactionRequests, _ ...grpc.CallOption) (*metamorph_api.TransactionStatuses, error) {
					return tc.putTxStatus, tc.putTxErr
				},
				GetTransactionStatusFunc: func(_ context.Context, _ *metamorph_api.TransactionStatusRequest, _ ...grpc.CallOption) (*metamorph_api.TransactionStatus, error) {
					return tc.getTxStatus, tc.getTxErr
				},
			}

			opts := []func(client *metamorph.Metamorph){
				metamorph.WithClientNow(func() time.Time { return now }),
			}
			if tc.withMqClient {
				mqClient := &apiMocks.MessageQueueClientMock{
					PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error {
						return tc.publishSubmitTxErr
					},
				}
				opts = append(opts, metamorph.WithMqClient(mqClient))
			}

			client := metamorph.NewClient(apiClient, opts...)
			// When
			ctx := context.Background()
			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			statuses, err := client.SubmitTransactions(timeoutCtx, sdkTx.Transactions{tx1, tx2, tx3}, tc.options)
			// Then
			require.Equal(t, tc.expectedStatuses, statuses)

			if tc.expectedErrorStr != "" {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestClient_GetTransaction(t *testing.T) {
	// Define test cases
	tt := []struct {
		name           string
		txID           string
		mockResp       *metamorph_api.Transaction
		mockErr        error
		expectedData   []byte
		expectedErrStr string
	}{
		{
			name:         "success - transaction found",
			txID:         "testTxID",
			mockResp:     &metamorph_api.Transaction{Txid: "testTxID", RawTx: []byte{0x01, 0x02, 0x03}},
			expectedData: []byte{0x01, 0x02, 0x03},
		},
		{
			name:           "error - transaction not found",
			txID:           "missingTxID",
			mockResp:       nil,
			mockErr:        errors.New("failed to get transaction"),
			expectedErrStr: "failed to get transaction",
		},
		{
			name:           "error - transaction is nil ",
			txID:           "nilTxID",
			mockResp:       nil,
			mockErr:        nil,
			expectedData:   nil,
			expectedErrStr: metamorph.ErrTransactionNotFound.Error(),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			mockClient := &apiMocks.MetaMorphAPIClientMock{
				GetTransactionFunc: func(_ context.Context, _ *metamorph_api.TransactionStatusRequest, _ ...grpc.CallOption) (*metamorph_api.Transaction, error) {
					return tc.mockResp, tc.mockErr
				},
			}

			client := metamorph.NewClient(mockClient)

			// When
			txData, err := client.GetTransaction(context.Background(), tc.txID)

			// Then
			if tc.expectedErrStr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErrStr)
				require.Nil(t, txData)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedData, txData)
			}

			// Ensure the mock was called once
			require.Equal(t, 1, len(mockClient.GetTransactionCalls()))
			require.Equal(t, tc.txID, mockClient.GetTransactionCalls()[0].In.Txid)
		})
	}
}

func TestClient_GetTransactions(t *testing.T) {
	tests := []struct {
		name             string
		txIDs            []string
		mockResponse     *metamorph_api.Transactions
		mockError        error
		expectedTxCount  int
		expectedErrorStr string
	}{
		{
			name:  "success - multiple transactions",
			txIDs: []string{"tx1", "tx2"},
			mockResponse: &metamorph_api.Transactions{
				Transactions: []*metamorph_api.Transaction{
					{
						Txid:        "tx1",
						RawTx:       []byte("transaction1"),
						BlockHeight: 100,
					},
					{
						Txid:        "tx2",
						RawTx:       []byte("transaction2"),
						BlockHeight: 101,
					},
				},
			},
			expectedTxCount: 2,
		},
		{
			name:             "error - transaction retrieval fails",
			txIDs:            []string{"tx1"},
			mockResponse:     nil,
			mockError:        errors.New("failed to retrieve transactions"),
			expectedErrorStr: "failed to retrieve transactions",
		},
		{
			name:             "error - transaction not found",
			txIDs:            []string{"tx1"},
			mockResponse:     nil,
			mockError:        errors.New("transaction not found"),
			expectedErrorStr: "transaction not found",
		},
		{
			name:            "txs is nil",
			txIDs:           []string{"tx1"},
			mockResponse:    nil,
			mockError:       nil,
			expectedTxCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			apiClient := &apiMocks.MetaMorphAPIClientMock{
				GetTransactionsFunc: func(_ context.Context, req *metamorph_api.TransactionsStatusRequest, _ ...grpc.CallOption) (*metamorph_api.Transactions, error) {
					require.Equal(t, tt.txIDs, req.TxIDs, "TxIDs do not match")
					return tt.mockResponse, tt.mockError
				},
			}

			client := metamorph.NewClient(apiClient)

			// When
			transactions, err := client.GetTransactions(context.Background(), tt.txIDs)

			// Then
			if tt.expectedErrorStr == "" {
				require.NoError(t, err)
				require.Len(t, transactions, tt.expectedTxCount)
				for i, tx := range transactions {
					require.Equal(t, tt.mockResponse.Transactions[i].Txid, tx.TxID, "Transaction ID mismatch")
					require.Equal(t, tt.mockResponse.Transactions[i].RawTx, tx.Bytes, "Raw transaction bytes mismatch")
					require.Equal(t, tt.mockResponse.Transactions[i].BlockHeight, tx.BlockHeight, "Block height mismatch")
				}
			} else {
				require.ErrorContains(t, err, tt.expectedErrorStr)
			}
		})
	}
}

func TestClient_GetTransactionStatus(t *testing.T) {
	tt := []struct {
		name           string
		txID           string
		mockResp       *metamorph_api.TransactionStatus
		mockErr        error
		expectedStatus *metamorph.TransactionStatus
		expectedErrStr string
	}{
		{
			name:           "error - client error",
			txID:           "testTxID1",
			mockResp:       nil,
			mockErr:        errors.New("client error"),
			expectedStatus: nil,
			expectedErrStr: "client error",
		},
		{
			name:           "error - transaction not found",
			txID:           "missingTxID",
			mockResp:       nil,
			mockErr:        nil,
			expectedStatus: nil,
			expectedErrStr: metamorph.ErrTransactionNotFound.Error(),
		},
		{
			name: "success - transaction status retrieved",
			txID: "testTxID2",
			mockResp: &metamorph_api.TransactionStatus{
				Txid:         "testTxID2",
				MerklePath:   "sampleMerklePath",
				Status:       metamorph_api.Status_MINED,
				BlockHash:    "sampleBlockHash",
				BlockHeight:  200,
				RejectReason: "none",
				CompetingTxs: []string{"competingTx1"},
			},
			expectedStatus: &metamorph.TransactionStatus{
				TxID:         "testTxID2",
				MerklePath:   "sampleMerklePath",
				Status:       metamorph_api.Status_MINED.String(),
				BlockHash:    "sampleBlockHash",
				BlockHeight:  200,
				ExtraInfo:    "none",
				CompetingTxs: []string{"competingTx1"},
				Timestamp:    time.Now().Unix(),
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			mockClient := &apiMocks.MetaMorphAPIClientMock{
				GetTransactionStatusFunc: func(_ context.Context, _ *metamorph_api.TransactionStatusRequest, _ ...grpc.CallOption) (*metamorph_api.TransactionStatus, error) {
					if tc.mockErr != nil {
						return nil, tc.mockErr
					}
					return tc.mockResp, nil
				},
			}

			now := time.Now().Unix()
			client := metamorph.NewClient(mockClient, metamorph.WithClientNow(func() time.Time { return time.Unix(now, 0) }))

			// When
			status, err := client.GetTransactionStatus(context.Background(), tc.txID)

			// Then
			if tc.expectedErrStr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErrStr)
				require.Nil(t, status)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedStatus, status)
			}

			require.Equal(t, 1, len(mockClient.GetTransactionStatusCalls()), "Unexpected number of GetTransactionStatus calls")
		})
	}
}

func TestClient_Health(t *testing.T) {
	tt := []struct {
		name           string
		mockError      error
		expectedErrStr string
	}{
		{
			name:           "success - health check passes",
			mockError:      nil,
			expectedErrStr: "",
		},
		{
			name:           "error - health check fails",
			mockError:      errors.New("health check error"),
			expectedErrStr: "health check error",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			mockClient := &apiMocks.MetaMorphAPIClientMock{
				HealthFunc: func(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*metamorph_api.HealthResponse, error) {
					return &metamorph_api.HealthResponse{}, tc.mockError
				},
			}

			client := metamorph.NewClient(mockClient)

			// When
			err := client.Health(context.Background())

			// Then
			if tc.expectedErrStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrStr)
			}

			require.Equal(t, 1, len(mockClient.HealthCalls()), "Unexpected number of Health calls")
		})
	}
}
