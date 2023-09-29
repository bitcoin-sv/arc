package metamorph

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	blockTxMock "github.com/bitcoin-sv/arc/metamorph/blocktx/mock"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	storeMock "github.com/bitcoin-sv/arc/metamorph/store/mock"
	"github.com/bitcoin-sv/arc/metamorph/store/sql"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils/stat"
)

const source = "localhost:8000"

func TestNewServer(t *testing.T) {
	t.Run("NewServer", func(t *testing.T) {
		server := NewServer(nil, nil, nil, nil, source)
		assert.IsType(t, &Server{}, server)
	})
}

func TestHealth(t *testing.T) {
	t.Run("Health", func(t *testing.T) {
		processor := NewProcessorMock()

		sentToNetworkStat := stat.NewAtomicStats()
		for i := 0; i < 10; i++ {
			sentToNetworkStat.AddDuration("test", 10*time.Millisecond)
		}

		processor.Stats = &ProcessorStats{
			StartTime:      time.Now(),
			UptimeMillis:   "2000ms",
			QueueLength:    136,
			QueuedCount:    356,
			SentToNetwork:  sentToNetworkStat,
			ChannelMapSize: 22,
		}
		server := NewServer(nil, nil, processor, nil, source)
		stats, err := server.Health(context.Background(), &emptypb.Empty{})
		assert.NoError(t, err)
		assert.Equal(t, processor.Stats.ChannelMapSize, stats.MapSize)
		assert.Equal(t, processor.Stats.QueuedCount, stats.Queued)
		assert.Equal(t, processor.Stats.SentToNetwork.GetMap()["test"].GetCount(), stats.Processed)
		assert.Equal(t, processor.Stats.QueueLength, stats.Waiting)
		assert.Equal(t, float32(10), stats.Average)
	})
}

func TestPutTransaction(t *testing.T) {
	t.Run("PutTransaction - ANNOUNCED", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		processor := NewProcessorMock()
		btc := NewBlockTxMock(source)
		btc.RegisterTransactionResponses = []interface{}{
			&blocktx_api.RegisterTransactionResponse{Source: source},
		}

		server := NewServer(nil, s, processor, btc, source)
		server.SetTimeout(100 * time.Millisecond)

		var txStatus *metamorph_api.TransactionStatus
		txRequest := &metamorph_api.TransactionRequest{
			RawTx: testdata.TX1RawBytes,
		}
		go func() {
			time.Sleep(10 * time.Millisecond)

			processor.GetProcessRequest(0).ResponseChannel <- processor_response.StatusAndError{
				Hash:   testdata.TX1Hash,
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			}
		}()

		txStatus, err = server.PutTransaction(context.Background(), txRequest)
		assert.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, txStatus.Status)
		assert.True(t, txStatus.TimedOut)
	})

	t.Run("invalid request", func(t *testing.T) {
		server := NewServer(nil, nil, nil, nil, source)

		txRequest := &metamorph_api.TransactionRequest{
			CallbackUrl: "api.callback.com",
		}

		_, err := server.PutTransaction(context.Background(), txRequest)
		assert.ErrorContains(t, err, "invalid URL [parse \"api.callback.com\": invalid URI for request]")
	})

	t.Run("PutTransaction - SEEN to network", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		processor := NewProcessorMock()
		btc := NewBlockTxMock(source)
		btc.RegisterTransactionResponses = []interface{}{
			&blocktx_api.RegisterTransactionResponse{Source: source},
		}

		server := NewServer(nil, s, processor, btc, source)

		var txStatus *metamorph_api.TransactionStatus
		txRequest := &metamorph_api.TransactionRequest{
			RawTx: testdata.TX1RawBytes,
		}
		go func() {
			time.Sleep(10 * time.Millisecond)
			processor.GetProcessRequest(0).ResponseChannel <- processor_response.StatusAndError{
				Hash:   testdata.TX1Hash,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			}
		}()
		txStatus, err = server.PutTransaction(context.Background(), txRequest)
		assert.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SEEN_ON_NETWORK, txStatus.Status)
		assert.False(t, txStatus.TimedOut)
	})

	t.Run("PutTransaction - Err", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		processor := NewProcessorMock()
		btc := NewBlockTxMock(source)
		btc.RegisterTransactionResponses = []interface{}{
			&blocktx_api.RegisterTransactionResponse{Source: source},
		}

		server := NewServer(nil, s, processor, btc, source)

		var txStatus *metamorph_api.TransactionStatus
		txRequest := &metamorph_api.TransactionRequest{
			RawTx: testdata.TX1RawBytes,
		}
		go func() {
			time.Sleep(10 * time.Millisecond)
			processor.GetProcessRequest(0).ResponseChannel <- processor_response.StatusAndError{
				Hash:   testdata.TX1Hash,
				Status: metamorph_api.Status_REJECTED,
				Err:    fmt.Errorf("some error"),
			}
		}()
		txStatus, err = server.PutTransaction(context.Background(), txRequest)
		assert.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_REJECTED, txStatus.Status)
		assert.Equal(t, "some error", txStatus.RejectReason)
		assert.False(t, txStatus.TimedOut)
	})

	t.Run("PutTransaction - Known tx", func(t *testing.T) {
		ctx := context.Background()
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)
		err = s.Set(ctx, testdata.TX1Hash[:], &store.StoreData{
			Hash:   testdata.TX1Hash,
			Status: metamorph_api.Status_SEEN_ON_NETWORK,
			RawTx:  testdata.TX1RawBytes,
		})
		require.NoError(t, err)

		processor := NewProcessorMock()
		btc := NewBlockTxMock(source)
		btc.RegisterTransactionResponses = []interface{}{
			&blocktx_api.RegisterTransactionResponse{Source: source},
		}

		server := NewServer(nil, s, processor, btc, source)

		txRequest := &metamorph_api.TransactionRequest{
			RawTx: testdata.TX1RawBytes,
		}

		var txStatus *metamorph_api.TransactionStatus
		txStatus, err = server.PutTransaction(ctx, txRequest)

		assert.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SEEN_ON_NETWORK, txStatus.Status)
		assert.False(t, txStatus.TimedOut)
	})
}

func TestServer_GetTransactionStatus(t *testing.T) {
	s, err := sql.New("sqlite_memory")
	require.NoError(t, err)
	setStoreTestData(t, s)

	tests := []struct {
		name    string
		req     *metamorph_api.TransactionStatusRequest
		want    *metamorph_api.TransactionStatus
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "GetTransactionStatus - not found",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: "a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118",
			},
			want: nil,
			wantErr: func(t assert.TestingT, err error, rest ...interface{}) bool {
				return assert.ErrorIs(t, err, store.ErrNotFound, rest)
			},
		},
		{
			name: "GetTransactionStatus - test.TX1",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: testdata.TX1,
			},
			want: &metamorph_api.TransactionStatus{
				StoredAt:    timestamppb.New(testdata.Time),
				AnnouncedAt: timestamppb.New(testdata.Time.Add(1 * time.Second)),
				MinedAt:     timestamppb.New(testdata.Time.Add(2 * time.Second)),
				Txid:        testdata.TX1,
				Status:      metamorph_api.Status_SENT_TO_NETWORK,
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(nil, s, nil, nil, source)
			got, err := server.GetTransactionStatus(context.Background(), tt.req)
			if !tt.wantErr(t, err, fmt.Sprintf("GetTransactionStatus(%v)", tt.req)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetTransactionStatus(%v)", tt.req)
		})
	}
}

func TestValidateCallbackURL(t *testing.T) {
	tt := []struct {
		name        string
		callbackURL string

		expectedErrorStr string
	}{
		{
			name:        "empty callback URL",
			callbackURL: "",
		},
		{
			name:        "valid callback URL",
			callbackURL: "http://api.callback.com",
		},
		{
			name:        "invalid callback URL",
			callbackURL: "api.callback.com",

			expectedErrorStr: "invalid URL [parse \"api.callback.com\": invalid URI for request]",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCallbackURL(tc.callbackURL)

			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// //go:generate moq -pkg metamorph -out ./processor_mock.go . ProcessorI ==> Todo: moq has a bug for creating a mock file in the same package. Currently fixed manually --> Create issue on moq github repo

func TestPutTransactions(t *testing.T) {
	hash0, err := chainhash.NewHashFromStr("9b58926ec7eed21ec2f3ca518d5fc0c6ccbf963e25c3e7ac496c99867d97599a")
	require.NoError(t, err)

	tx0, err := bt.NewTxFromString("010000000000000000ef016b51c656fb06639ea6c1c3642a5ede9ecf9f749b95cb47d4e57eda7a3953b1c64c0000006a47304402201ade53acd924e90c0aeabbf9085d075acb23c4712e7f728a23979a466ab55e19022047a85963ce2eddc21573b4a6c0e7ccfec44153e74f9d03d31f955ff486449240412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffe8030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01e7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
	require.Equal(t, tx0.TxID(), hash0.String())

	require.NoError(t, err)
	tx1, err := bt.NewTxFromString("010000000000000000ef016b51c656fb06639ea6c1c3642a5ede9ecf9f749b95cb47d4e57eda7a3953b1c6660000006b483045022100e6d888a31cabb7bd491da63c9378d550ab728e6f81aa1c9420e1e055123e4728022040fd7263f08ecb53a1c9dbbc074d4b36e34e8db2ce78fed012a517052befda2b412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffe8030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01e7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
	require.NoError(t, err)
	hash1, err := chainhash.NewHashFromStr("5d09daee7a648db6f99a7b678e9d64e6bf6867fb8a5f8818f4718b5a871fead1")
	require.NoError(t, err)
	require.Equal(t, tx1.TxID(), hash1.String())

	tx2, err := bt.NewTxFromString("010000000000000000ef016b51c656fb06639ea6c1c3642a5ede9ecf9f749b95cb47d4e57eda7a3953b1c6690000006a4730440220519b37c338888500e8299dd9afe462930352c95af1b436a29411b5eaaca7ec9c02204f821540a109323dbb36bd1d89bc057a435a4efbb5df7c3cae0d8522265cdd5c412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffe8030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01e7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
	require.NoError(t, err)
	hash2, err := chainhash.NewHashFromStr("337bf4982dd12f399c1f20a7806c8005255355d8df84621062f572571f52f03b")
	require.NoError(t, err)
	require.Equal(t, tx2.TxID(), hash2.String())

	tt := []struct {
		name              string
		processorResponse map[string]*processor_response.StatusAndError
		transactionFound  map[int]*store.StoreData
		requests          *metamorph_api.TransactionRequests

		expectedErrorStr                         string
		expectedStatuses                         *metamorph_api.TransactionStatuses
		expectedProcessorSetCalls                int
		expectedProcessorProcessTransactionCalls int
	}{
		{
			name: "single new transaction response seen on network - wait for sent to network status",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{
					{
						RawTx:         tx0.Bytes(),
						WaitForStatus: metamorph_api.Status_SENT_TO_NETWORK,
					},
				},
			},
			processorResponse: map[string]*processor_response.StatusAndError{hash0.String(): {
				Hash:   hash0,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
				Err:    nil,
			}},

			expectedProcessorSetCalls:                1,
			expectedProcessorProcessTransactionCalls: 1,
			expectedStatuses: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:   hash0.String(),
						Status: metamorph_api.Status_SEEN_ON_NETWORK,
					},
				},
			},
		},
		{
			name: "single new transaction response with error",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{{RawTx: tx0.Bytes()}},
			},
			processorResponse: map[string]*processor_response.StatusAndError{hash0.String(): {
				Hash:   hash0,
				Status: metamorph_api.Status_STORED,
				Err:    errors.New("unable to process transaction"),
			}},

			expectedProcessorSetCalls:                1,
			expectedProcessorProcessTransactionCalls: 1,
			expectedStatuses: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:         hash0.String(),
						Status:       metamorph_api.Status_STORED,
						RejectReason: "unable to process transaction",
					},
				},
			},
		},
		{
			name: "single new transaction no response",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{{RawTx: tx0.Bytes()}},
			},

			expectedProcessorSetCalls:                1,
			expectedProcessorProcessTransactionCalls: 1,
			expectedStatuses: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:     hash0.String(),
						Status:   metamorph_api.Status_STORED,
						TimedOut: true,
					},
				},
			},
		},
		{
			name: "batch of 3 transactions - 2nd already stored",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{
					{
						RawTx:         tx0.Bytes(),
						WaitForStatus: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					}, {
						RawTx:         tx1.Bytes(),
						WaitForStatus: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					}, {
						RawTx:         tx2.Bytes(),
						WaitForStatus: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					},
				},
			},
			transactionFound: map[int]*store.StoreData{1: {
				Status:      metamorph_api.Status_SENT_TO_NETWORK,
				Hash:        hash1,
				StoredAt:    time.Time{},
				AnnouncedAt: time.Time{},
				MinedAt:     time.Time{},
			}},
			processorResponse: map[string]*processor_response.StatusAndError{
				hash0.String(): {
					Hash:   hash0,
					Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					Err:    nil,
				},
				hash2.String(): {
					Hash:   hash2,
					Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
					Err:    nil,
				},
			},

			expectedProcessorSetCalls:                2,
			expectedProcessorProcessTransactionCalls: 2,
			expectedStatuses: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:   hash0.String(),
						Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					},
					{
						Txid:        hash1.String(),
						Status:      metamorph_api.Status_SENT_TO_NETWORK,
						BlockHash:   "<nil>",
						StoredAt:    timestamppb.New(time.Time{}),
						AnnouncedAt: timestamppb.New(time.Time{}),
						MinedAt:     timestamppb.New(time.Time{}),
					},
					{
						Txid:   hash2.String(),
						Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			getCounter := 0
			metamorphStore := &storeMock.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					defer func() { getCounter++ }()

					storeData, found := tc.transactionFound[getCounter]
					if found {
						return storeData, nil
					}

					return nil, nil
				},
			}

			btc := &blockTxMock.ClientIMock{
				RegisterTransactionFunc: func(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (*blocktx_api.RegisterTransactionResponse, error) {
					resp := &blocktx_api.RegisterTransactionResponse{
						Source: "localhost:8000",
					}
					return resp, nil
				},
			}

			processor := &ProcessorIMock{
				SetFunc: func(req *ProcessorRequest) error {
					return nil
				},
				ProcessTransactionFunc: func(req *ProcessorRequest) {
					resp, found := tc.processorResponse[req.Hash.String()]
					if found {
						req.ResponseChannel <- *resp
					}
				},
			}
			server := NewServer(nil, metamorphStore, processor, btc, source)

			server.SetTimeout(5 * time.Second)
			statuses, err := server.PutTransactions(context.Background(), tc.requests)
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedProcessorSetCalls, len(processor.calls.Set))
			require.Equal(t, tc.expectedProcessorProcessTransactionCalls, len(processor.calls.ProcessTransaction))

			for i := 0; i < len(tc.expectedStatuses.Statuses); i++ {
				expected := tc.expectedStatuses.Statuses[i]
				status := statuses.Statuses[i]
				require.Equal(t, expected, status)
			}
		})
	}
}
