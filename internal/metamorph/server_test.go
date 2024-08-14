package metamorph_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewServer(t *testing.T) {
	t.Run("NewServer", func(t *testing.T) {
		server := metamorph.NewServer(nil, nil)
		assert.IsType(t, &metamorph.Server{}, server)
	})
}

func TestHealth(t *testing.T) {
	t.Run("Health", func(t *testing.T) {
		processor := &mocks.ProcessorIMock{}
		processor.GetProcessorMapSizeFunc = func() int { return 22 }
		processor.GetPeersFunc = func() []p2p.PeerI {
			return []p2p.PeerI{}
		}

		server := metamorph.NewServer(nil, processor)
		stats, err := server.Health(context.Background(), &emptypb.Empty{})
		assert.NoError(t, err)
		assert.Equal(t, int32(22), stats.GetMapSize())
	})
}

func TestPutTransaction(t *testing.T) {
	testCases := []struct {
		name              string
		processorResponse metamorph.StatusAndError
		waitForStatus     metamorph_api.Status

		expectedStatus         metamorph_api.Status
		expectedRejectedReason string
		expectedCompetingTxs   []string
		expectedTimeout        bool
	}{
		{
			name: "announced to network",
			processorResponse: metamorph.StatusAndError{
				Hash:   testdata.TX1Hash,
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
			waitForStatus: metamorph_api.Status_SEEN_ON_NETWORK,

			expectedStatus:  metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			expectedTimeout: true,
		},
		{
			name: "seen on network",
			processorResponse: metamorph.StatusAndError{
				Hash:   testdata.TX1Hash,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			waitForStatus: metamorph_api.Status_SEEN_ON_NETWORK,

			expectedStatus:  metamorph_api.Status_SEEN_ON_NETWORK,
			expectedTimeout: false,
		},
		{
			name: "double spend attempted",
			processorResponse: metamorph.StatusAndError{
				Hash:         testdata.TX1Hash,
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"1234"},
			},
			waitForStatus: metamorph_api.Status_SEEN_ON_NETWORK,

			expectedStatus:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
			expectedCompetingTxs: []string{"1234"},
			expectedTimeout:      false,
		},
		{
			name: "error",
			processorResponse: metamorph.StatusAndError{
				Hash:   testdata.TX1Hash,
				Status: metamorph_api.Status_REJECTED,
				Err:    fmt.Errorf("some error"),
			},
			waitForStatus: metamorph_api.Status_SEEN_ON_NETWORK,

			expectedStatus:         metamorph_api.Status_REJECTED,
			expectedRejectedReason: "some error",
			expectedTimeout:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &storeMocks.MetamorphStoreMock{}

			processor := &mocks.ProcessorIMock{
				ProcessTransactionFunc: func(ctx context.Context, req *metamorph.ProcessorRequest) {
					time.Sleep(10 * time.Millisecond)
					req.ResponseChannel <- tc.processorResponse
				},
			}

			server := metamorph.NewServer(s, processor, metamorph.WithMaxTimeoutDefault(100*time.Millisecond))

			txRequest := &metamorph_api.TransactionRequest{
				RawTx:         testdata.TX1Raw.Bytes(),
				WaitForStatus: tc.waitForStatus,
			}

			txStatus, err := server.PutTransaction(context.Background(), txRequest)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, txStatus.GetStatus())
			assert.Equal(t, tc.expectedRejectedReason, txStatus.GetRejectReason())
			assert.Equal(t, tc.expectedCompetingTxs, txStatus.CompetingTxs)

			if tc.expectedTimeout {
				assert.True(t, txStatus.GetTimedOut())
			}
		})
	}
}

func TestServer_GetTransactionStatus(t *testing.T) {
	errFailedToGetTxData := errors.New("failed to get transaction data")

	tests := []struct {
		name               string
		req                *metamorph_api.TransactionStatusRequest
		getTxMerklePathErr error
		getErr             error
		status             metamorph_api.Status
		competingTxs       []string

		want    *metamorph_api.TransactionStatus
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "GetTransactionStatus - error: not found",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: "a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118",
			},
			getErr: store.ErrNotFound,

			want: nil,
			wantErr: func(t assert.TestingT, err error, rest ...interface{}) bool {
				return assert.ErrorIs(t, err, metamorph.ErrNotFound, rest...)
			},
		},
		{
			name: "GetTransactionStatus - error: failed to get tx data",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: "a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118",
			},
			getErr: errFailedToGetTxData,

			want: nil,
			wantErr: func(t assert.TestingT, err error, rest ...interface{}) bool {
				return assert.ErrorIs(t, err, errFailedToGetTxData, rest...)
			},
		},
		{
			name: "GetTransactionStatus - test.TX1",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: testdata.TX1Hash.String(),
			},
			status: metamorph_api.Status_SENT_TO_NETWORK,

			want: &metamorph_api.TransactionStatus{
				StoredAt:    timestamppb.New(testdata.Time),
				AnnouncedAt: timestamppb.New(testdata.Time.Add(1 * time.Second)),
				MinedAt:     timestamppb.New(testdata.Time.Add(2 * time.Second)),
				Txid:        testdata.TX1Hash.String(),
				Status:      metamorph_api.Status_SENT_TO_NETWORK,
				MerklePath:  "00000",
			},
			wantErr: assert.NoError,
		},
		{
			name: "GetTransactionStatus - double spend attempted",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: testdata.TX1Hash.String(),
			},
			status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
			competingTxs: []string{"1234"},

			want: &metamorph_api.TransactionStatus{
				StoredAt:     timestamppb.New(testdata.Time),
				AnnouncedAt:  timestamppb.New(testdata.Time.Add(1 * time.Second)),
				MinedAt:      timestamppb.New(testdata.Time.Add(2 * time.Second)),
				Txid:         testdata.TX1Hash.String(),
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"1234"},
				MerklePath:   "00000",
			},
			wantErr: assert.NoError,
		},
		{
			name: "GetTransactionStatus - mined - previously double spend attempted",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: testdata.TX1Hash.String(),
			},
			status:       metamorph_api.Status_MINED,
			competingTxs: []string{"1234"},

			want: &metamorph_api.TransactionStatus{
				StoredAt:     timestamppb.New(testdata.Time),
				AnnouncedAt:  timestamppb.New(testdata.Time.Add(1 * time.Second)),
				MinedAt:      timestamppb.New(testdata.Time.Add(2 * time.Second)),
				Txid:         testdata.TX1Hash.String(),
				Status:       metamorph_api.Status_MINED,
				CompetingTxs: []string{},
				RejectReason: "previously double spend attempted",
				MerklePath:   "00000",
			},
			wantErr: assert.NoError,
		},
		{
			name: "GetTransactionStatus - test.TX1 - error",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: testdata.TX1Hash.String(),
			},
			status:             metamorph_api.Status_SENT_TO_NETWORK,
			getTxMerklePathErr: errors.New("failed to get tx merkle path"),

			want: &metamorph_api.TransactionStatus{
				StoredAt:    timestamppb.New(testdata.Time),
				AnnouncedAt: timestamppb.New(testdata.Time.Add(1 * time.Second)),
				MinedAt:     timestamppb.New(testdata.Time.Add(2 * time.Second)),
				Txid:        testdata.TX1Hash.String(),
				Status:      metamorph_api.Status_SENT_TO_NETWORK,
				MerklePath:  "00000",
			},
			wantErr: assert.NoError,
		},
		{
			name: "GetTransactionStatus - test.TX1 - tx not found for Merkle path",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: testdata.TX1Hash.String(),
			},
			status:             metamorph_api.Status_MINED,
			getTxMerklePathErr: blocktx.ErrMerklePathNotFoundForTransaction,

			want: &metamorph_api.TransactionStatus{
				StoredAt:    timestamppb.New(testdata.Time),
				AnnouncedAt: timestamppb.New(testdata.Time.Add(1 * time.Second)),
				MinedAt:     timestamppb.New(testdata.Time.Add(2 * time.Second)),
				Txid:        testdata.TX1Hash.String(),
				Status:      metamorph_api.Status_MINED,
				MerklePath:  "00000",
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					data := &store.StoreData{
						StoredAt:     testdata.Time,
						AnnouncedAt:  testdata.Time.Add(1 * time.Second),
						MinedAt:      testdata.Time.Add(2 * time.Second),
						Hash:         testdata.TX1Hash,
						Status:       tt.status,
						CompetingTxs: tt.competingTxs,
						Callbacks:    []store.StoreCallback{{CallbackURL: "https://test.com", CallbackToken: "token"}},
						MerklePath:   "00000",
					}
					return data, tt.getErr
				},
			}

			server := metamorph.NewServer(metamorphStore, nil)
			got, err := server.GetTransactionStatus(context.Background(), tt.req)
			if !tt.wantErr(t, err, fmt.Sprintf("GetTransactionStatus(%v)", tt.req)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetTransactionStatus(%v)", tt.req)
		})
	}
}

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
		processorResponse map[string]*metamorph.StatusAndError
		transactionFound  map[int]*store.StoreData
		requests          *metamorph_api.TransactionRequests
		getErr            error

		expectedErrorStr                         string
		expectedStatuses                         *metamorph_api.TransactionStatuses
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
			processorResponse: map[string]*metamorph.StatusAndError{hash0.String(): {
				Hash:   hash0,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
				Err:    nil,
			}},

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
			name: "single new transaction - double spend attempted",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{
					{
						RawTx:         tx0.Bytes(),
						WaitForStatus: metamorph_api.Status_SEEN_ON_NETWORK,
					},
				},
			},
			processorResponse: map[string]*metamorph.StatusAndError{hash0.String(): {
				Hash:         hash0,
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				Err:          nil,
				CompetingTxs: []string{"1234"},
			}},

			expectedProcessorProcessTransactionCalls: 1,
			expectedStatuses: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:         hash0.String(),
						Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
						CompetingTxs: []string{"1234"},
					},
				},
			},
		},
		{
			name: "single new transaction response with error",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{
					{
						RawTx:         tx0.Bytes(),
						WaitForStatus: metamorph_api.Status_STORED,
					},
				},
			},
			processorResponse: map[string]*metamorph.StatusAndError{hash0.String(): {
				Hash:   hash0,
				Status: metamorph_api.Status_STORED,
				Err:    errors.New("unable to process transaction"),
			}},

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
			name: "single new transaction - wait for received status",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{{
					RawTx:         tx0.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
				}},
			},

			expectedProcessorProcessTransactionCalls: 1,
			expectedStatuses: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:     hash0.String(),
						Status:   metamorph_api.Status_RECEIVED,
						TimedOut: false,
					},
				},
			},
		},
		{
			name: "single new transaction - time out",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{{RawTx: tx0.Bytes()}},
			},

			expectedProcessorProcessTransactionCalls: 1,
			expectedStatuses: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:     hash0.String(),
						Status:   metamorph_api.Status_RECEIVED,
						TimedOut: true,
					},
				},
			},
		},
		{
			name: "batch of 3 transactions",
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
			processorResponse: map[string]*metamorph.StatusAndError{
				hash0.String(): {
					Hash:   hash0,
					Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					Err:    nil,
				},
				hash1.String(): {
					Hash:   hash1,
					Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					Err:    nil,
				},
				hash2.String(): {
					Hash:   hash2,
					Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
					Err:    nil,
				},
			},

			expectedProcessorProcessTransactionCalls: 3,
			expectedStatuses: &metamorph_api.TransactionStatuses{
				Statuses: []*metamorph_api.TransactionStatus{
					{
						Txid:   hash0.String(),
						Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					},
					{
						Txid:   hash1.String(),
						Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					},
					{
						Txid:   hash2.String(),
						Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
					},
				},
			},
		},
		{
			name: "failed to get tx",
			requests: &metamorph_api.TransactionRequests{
				Transactions: []*metamorph_api.TransactionRequest{
					{
						RawTx:         tx0.Bytes(),
						WaitForStatus: metamorph_api.Status_SENT_TO_NETWORK,
					},
				},
			},
			processorResponse: map[string]*metamorph.StatusAndError{hash0.String(): {
				Hash:   hash0,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
				Err:    nil,
			}},
			getErr: errors.New("failed to get tx"),

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
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			processor := &mocks.ProcessorIMock{
				ProcessTransactionFunc: func(ctx context.Context, req *metamorph.ProcessorRequest) {
					resp, found := tc.processorResponse[req.Data.Hash.String()]
					if found {
						req.ResponseChannel <- *resp
					}
				},
			}

			serverLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			server := metamorph.NewServer(nil, processor, metamorph.WithLogger(serverLogger), metamorph.WithMaxTimeoutDefault(5*time.Second))

			statuses, err := server.PutTransactions(context.Background(), tc.requests)
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedProcessorProcessTransactionCalls, len(processor.ProcessTransactionCalls()))

			for i := 0; i < len(tc.expectedStatuses.GetStatuses()); i++ {
				expected := tc.expectedStatuses.GetStatuses()[i]
				status := statuses.GetStatuses()[i]
				require.Equal(t, expected, status)
			}
		})
	}
}

func TestSetUnlockedbyName(t *testing.T) {
	tt := []struct {
		name            string
		recordsAffected int64
		errSetUnlocked  error

		expectedRecordsAffected int
		expectedErrorStr        string
	}{
		{
			name:            "success",
			recordsAffected: 5,

			expectedRecordsAffected: 5,
		},
		{
			name: "error",

			errSetUnlocked:   errors.New("failed to set unlocked"),
			expectedErrorStr: "failed to set unlocked",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					return &store.StoreData{}, nil
				},
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) {
					return tc.recordsAffected, tc.errSetUnlocked
				},
			}

			server := metamorph.NewServer(metamorphStore, nil)
			response, err := server.SetUnlockedByName(context.Background(), &metamorph_api.SetUnlockedByNameRequest{
				Name: "test",
			})

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.Equal(t, tc.expectedRecordsAffected, int(response.GetRecordsAffected()))
		})
	}
}

func TestStartGRPCServer(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "start and shutdown",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					return &store.StoreData{}, nil
				},
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
			}

			processor := &mocks.ProcessorIMock{
				ShutdownFunc: func() {},
			}
			server := metamorph.NewServer(metamorphStore, processor)
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			err := server.StartGRPCServer("localhost:7000", 10000, "", logger)
			require.NoError(t, err)

			time.Sleep(50 * time.Millisecond)

			server.Shutdown()
		})
	}
}

func Test_GetTransactions(t *testing.T) {
	tcs := []struct {
		name    string
		request *metamorph_api.TransactionsStatusRequest

		getFromStoreErr           error
		getFromStoreResponseCount int
	}{
		{
			name: "found all - success",
			request: &metamorph_api.TransactionsStatusRequest{
				TxIDs: []string{
					testdata.TX1Hash.String(),
					testdata.TX2Hash.String(),
				},
			},
			getFromStoreResponseCount: 2,
		},
		{
			name: "not found - success, return empty array",
			request: &metamorph_api.TransactionsStatusRequest{
				TxIDs: []string{
					testdata.TX1Hash.String(),
					testdata.TX2Hash.String(),
				},
			},
		},
		{
			name: "failed to get data from the store",
			request: &metamorph_api.TransactionsStatusRequest{
				TxIDs: []string{
					testdata.TX1Hash.String(),
					testdata.TX2Hash.String(),
				},
			},
			getFromStoreErr: errors.New("test error"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// when
			store := storeMocks.MetamorphStoreMock{
				GetManyFunc: func(ctx context.Context, keys [][]byte) ([]*store.StoreData, error) {
					if tc.getFromStoreErr != nil {
						return nil, tc.getFromStoreErr
					}

					res := make([]*store.StoreData, 0)
					for _, hash := range tc.request.TxIDs {
						h, _ := chainhash.NewHashFromStr(hash)
						d := store.StoreData{
							Hash: h,
						}

						res = append(res, &d)
					}

					res = res[:tc.getFromStoreResponseCount]
					return res, nil
				},
			}

			sut := metamorph.NewServer(&store, nil)

			// then
			res, err := sut.GetTransactions(context.TODO(), tc.request)

			// assert
			if tc.getFromStoreErr != nil {
				require.Equal(t, tc.getFromStoreErr, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Len(t, res.Transactions, tc.getFromStoreResponseCount)
			}
		})
	}
}
