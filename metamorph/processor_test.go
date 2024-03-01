package metamorph_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/mocks"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:generate moq -pkg mocks -out ./mocks/store_mock.go ./store/ MetamorphStore
//go:generate moq -pkg mocks -out ./mocks/message_queue_mock.go . MessageQueueClient
//go:generate moq -pkg mocks -out ./mocks/http_client_mock.go . HttpClient
//go:generate moq -pkg mocks -out ./mocks/peer_manager_mock.go . PeerManager

func TestNewProcessor(t *testing.T) {
	mtmStore := &mocks.MetamorphStoreMock{
		GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
			return &store.StoreData{Hash: testdata.TX2Hash}, nil
		},
		SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error { return nil },
	}

	pm := &mocks.PeerManagerMock{}

	tt := []struct {
		name  string
		store store.MetamorphStore
		pm    p2p.PeerManagerI

		expectedErrorStr        string
		expectedNonNilProcessor bool
	}{
		{
			name:                    "success",
			store:                   mtmStore,
			pm:                      pm,
			expectedNonNilProcessor: true,
		},
		{
			name:  "no store",
			store: nil,
			pm:    pm,

			expectedErrorStr: "store cannot be nil",
		},
		{
			name:  "no pm",
			store: mtmStore,
			pm:    nil,

			expectedErrorStr: "peer manager cannot be nil",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			processor, err := metamorph.NewProcessor(tc.store, tc.pm,
				metamorph.WithCacheExpiryTime(time.Second*5),
				metamorph.WithProcessorLogger(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: metamorph.LogLevelDefault}))),
			)
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			defer processor.Shutdown()

			if tc.expectedNonNilProcessor && processor == nil {
				t.Error("Expected a non-nil Processor")
			}
		})
	}
}

func TestLoadUnmined(t *testing.T) {
	storedAt := time.Date(2023, 10, 3, 5, 0, 0, 0, time.UTC)

	tt := []struct {
		name              string
		storedData        []*store.StoreData
		transactionBlocks *blocktx_api.TransactionBlocks
		maxMonitoredTxs   int64
		getUnminedErr     error

		expectedGetTransactionBlocksCalls int
		expectedItemTxHashesFinal         []*chainhash.Hash
	}{
		{
			name: "no unmined transactions loaded",

			expectedGetTransactionBlocksCalls: 0,
			expectedItemTxHashesFinal:         []*chainhash.Hash{testdata.TX3Hash, testdata.TX4Hash},
		},
		{
			name: "load 2 unmined transactions, none mined",
			storedData: []*store.StoreData{
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX1Hash,
					Status:      metamorph_api.Status_ANNOUNCED_TO_NETWORK,
				},
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX2Hash,
					Status:      metamorph_api.Status_STORED,
				},
			},
			transactionBlocks: &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{{
				BlockHash:       nil,
				BlockHeight:     0,
				TransactionHash: nil,
			}}},
			maxMonitoredTxs: 5,

			expectedGetTransactionBlocksCalls: 1,
			expectedItemTxHashesFinal:         []*chainhash.Hash{testdata.TX1Hash, testdata.TX2Hash, testdata.TX3Hash, testdata.TX4Hash},
		},
		{
			name:            "load 2 unmined transactions, failed to get unmined",
			storedData:      []*store.StoreData{},
			getUnminedErr:   errors.New("failed to get unmined"),
			maxMonitoredTxs: 5,

			expectedItemTxHashesFinal: []*chainhash.Hash{testdata.TX3Hash, testdata.TX4Hash},
		},
		{
			name: "load 2 unmined transactions, limit reached",
			storedData: []*store.StoreData{
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX1Hash,
					Status:      metamorph_api.Status_ANNOUNCED_TO_NETWORK,
				},
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX2Hash,
					Status:      metamorph_api.Status_STORED,
				},
			},
			maxMonitoredTxs: 1,

			expectedGetTransactionBlocksCalls: 0,
			expectedItemTxHashesFinal:         []*chainhash.Hash{testdata.TX3Hash, testdata.TX4Hash},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			pm := &mocks.PeerManagerMock{}

			mtmStore := &mocks.MetamorphStoreMock{
				GetUnminedFunc: func(ctx context.Context, since time.Time, limit int64) ([]*store.StoreData, error) {
					return tc.storedData, tc.getUnminedErr
				},
				SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error {
					require.ElementsMatch(t, tc.expectedItemTxHashesFinal, hashes)
					require.Equal(t, len(tc.expectedItemTxHashesFinal), len(hashes))
					return nil
				},
				UpdateMinedFunc: func(ctx context.Context, txsBlocks *blocktx_api.TransactionBlocks) ([]*store.StoreData, error) {
					return nil, nil
				},
			}

			processor, err := metamorph.NewProcessor(mtmStore, pm,
				metamorph.WithCacheExpiryTime(time.Hour*24),
				metamorph.WithNow(func() time.Time {
					return storedAt.Add(1 * time.Hour)
				}),
				metamorph.WithDataRetentionPeriod(time.Hour*24),
				metamorph.WithMaxMonitoredTxs(tc.maxMonitoredTxs),
			)
			require.NoError(t, err)
			defer processor.Shutdown()
			require.Equal(t, 0, processor.ProcessorResponseMap.Len())

			processor.ProcessorResponseMap.Set(testdata.TX3Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX3Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK))
			processor.ProcessorResponseMap.Set(testdata.TX4Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX4Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK))

			processor.LoadUnmined()

			time.Sleep(time.Millisecond * 200)

			allItemHashes := make([]*chainhash.Hash, 0, len(processor.ProcessorResponseMap.Items()))

			for i, item := range processor.ProcessorResponseMap.Items() {
				require.Equal(t, i, *item.Hash)
				allItemHashes = append(allItemHashes, item.Hash)
			}

			require.ElementsMatch(t, tc.expectedItemTxHashesFinal, allItemHashes)
		})
	}
}

func TestProcessTransaction(t *testing.T) {
	tt := []struct {
		name            string
		storeData       *store.StoreData
		storeDataGetErr error

		expectedResponseMapItems int
		expectedResponses        []metamorph_api.Status
		expectedSetCalls         int
	}{
		{
			name:            "record not found",
			storeData:       nil,
			storeDataGetErr: metamorph.ErrNotFound,

			expectedResponses: []metamorph_api.Status{
				metamorph_api.Status_STORED,
				metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
			expectedResponseMapItems: 1,
			expectedSetCalls:         1,
		},
		{
			name: "record found",
			storeData: &store.StoreData{
				Hash:         testdata.TX1Hash,
				Status:       metamorph_api.Status_REJECTED,
				RejectReason: "missing inputs",
			},
			storeDataGetErr: nil,

			expectedResponses: []metamorph_api.Status{
				metamorph_api.Status_REJECTED,
			},
			expectedSetCalls: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := &mocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					require.Equal(t, testdata.TX1Hash[:], key)

					return tc.storeData, tc.storeDataGetErr
				},
				SetFunc: func(ctx context.Context, key []byte, value *store.StoreData) error {
					require.Equal(t, testdata.TX1Hash[:], key)

					return nil
				},
			}
			pm := &mocks.PeerManagerMock{AnnounceTransactionFunc: func(txHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI {
				require.True(t, testdata.TX1Hash.IsEqual(txHash))
				return nil
			}}

			publisher := &mocks.MessageQueueClientMock{
				PublishRegisterTxsFunc: func(hash []byte) error {
					return nil
				},
			}

			processor, err := metamorph.NewProcessor(s, pm, metamorph.WithMessageQueueClient(publisher))
			require.NoError(t, err)
			require.Equal(t, 0, processor.ProcessorResponseMap.Len())

			responseChannel := make(chan processor_response.StatusAndError)

			var wg sync.WaitGroup
			wg.Add(len(tc.expectedResponses))
			go func() {
				n := 0
				for response := range responseChannel {
					status := response.Status
					require.Equal(t, testdata.TX1Hash, response.Hash)
					require.Equalf(t, tc.expectedResponses[n], status, "Iteration %d: Expected %s, got %s", n, tc.expectedResponses[n].String(), status.String())
					wg.Done()
					n++
				}
			}()

			processor.ProcessTransaction(context.TODO(), &metamorph.ProcessorRequest{
				Data: &store.StoreData{
					Hash: testdata.TX1Hash,
				},
				ResponseChannel: responseChannel,
			})
			wg.Wait()

			if tc.expectedResponseMapItems > 0 {
				require.Equal(t, tc.expectedResponseMapItems, processor.ProcessorResponseMap.Len())
				items := processor.ProcessorResponseMap.Items()
				require.Equal(t, testdata.TX1Hash, items[*testdata.TX1Hash].Hash)
				require.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, items[*testdata.TX1Hash].GetStatus())

				require.Len(t, pm.AnnounceTransactionCalls(), 1)

			}

			require.Equal(t, tc.expectedSetCalls, len(s.SetCalls()))
		})
	}
}

func TestSendStatusForTransaction(t *testing.T) {
	type input struct {
		hash      *chainhash.Hash
		newStatus metamorph_api.Status
		statusErr error

		txResponseHash      *chainhash.Hash
		txResponseHashValue *processor_response.ProcessorResponse
	}

	tt := []struct {
		name       string
		inputs     []input
		updateErr  error
		updateResp [][]*store.StoreData

		expectedUpdateStatusCalls int
		expectedCallbacks         int
	}{
		{
			name: "tx not in response map - no update",
			inputs: []input{
				{
					hash:                testdata.TX1Hash,
					newStatus:           metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					statusErr:           nil,
					txResponseHash:      nil,
					txResponseHashValue: nil,
				},
			},

			expectedUpdateStatusCalls: 0,
		},
		{
			name: "tx in response map - current status REJECTED, new status SEEN_ON_NETWORK - no update",
			inputs: []input{
				{
					hash:                testdata.TX1Hash,
					newStatus:           metamorph_api.Status_SEEN_ON_NETWORK,
					statusErr:           nil,
					txResponseHash:      testdata.TX1Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_REJECTED),
				},
			},

			expectedUpdateStatusCalls: 0,
		},
		{
			name: "new status MINED - update error",
			inputs: []input{
				{
					hash:                testdata.TX1Hash,
					newStatus:           metamorph_api.Status_MINED,
					txResponseHash:      testdata.TX1Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SEEN_ON_NETWORK),
				},
			},
			updateErr: errors.New("failed to update status"),

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "status update - success",
			inputs: []input{
				{
					hash:                testdata.TX1Hash,
					newStatus:           metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					statusErr:           nil,
					txResponseHash:      testdata.TX1Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_STORED),
				},
				{
					hash:                testdata.TX2Hash,
					newStatus:           metamorph_api.Status_REJECTED,
					statusErr:           errors.New("missing inputs"),
					txResponseHash:      testdata.TX2Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_SENT_TO_NETWORK),
				},
				{
					hash:                testdata.TX3Hash,
					newStatus:           metamorph_api.Status_SENT_TO_NETWORK,
					txResponseHash:      testdata.TX3Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX3Hash, metamorph_api.Status_SENT_TO_NETWORK),
				},
				{
					hash:                testdata.TX4Hash,
					newStatus:           metamorph_api.Status_ACCEPTED_BY_NETWORK,
					txResponseHash:      testdata.TX4Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX4Hash, metamorph_api.Status_SENT_TO_NETWORK),
				},
				{
					hash:                testdata.TX5Hash,
					newStatus:           metamorph_api.Status_SEEN_ON_NETWORK,
					txResponseHash:      testdata.TX5Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX5Hash, metamorph_api.Status_REQUESTED_BY_NETWORK),
				},
				{
					hash:                testdata.TX6Hash,
					newStatus:           metamorph_api.Status_REQUESTED_BY_NETWORK,
					txResponseHash:      testdata.TX6Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX6Hash, metamorph_api.Status_STORED),
				},
			},
			updateResp: [][]*store.StoreData{
				{
					{
						Hash:              testdata.TX1Hash,
						Status:            metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL,
						FullStatusUpdates: true,
						CallbackUrl:       "http://callback.com",
					},
				},
				{
					{
						Hash:              testdata.TX5Hash,
						Status:            metamorph_api.Status_SEEN_ON_NETWORK,
						RejectReason:      "",
						FullStatusUpdates: true,
						CallbackUrl:       "http://callback.com",
					},
				},
			},

			expectedCallbacks:         2,
			expectedUpdateStatusCalls: 2,
		},
		{
			name: "multiple updates - with duplicates",
			inputs: []input{
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_REQUESTED_BY_NETWORK,

					txResponseHash:      testdata.TX1Hash,
					txResponseHashValue: processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK),
				},
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_SEEN_ON_NETWORK,
				},
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_SENT_TO_NETWORK,
				},
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_ACCEPTED_BY_NETWORK,
				},
			},
			updateResp: [][]*store.StoreData{
				{
					{
						Hash:              testdata.TX1Hash,
						CallbackUrl:       "http://callback.com",
						FullStatusUpdates: true,
						Status:            metamorph_api.Status_SEEN_ON_NETWORK,
					},
				},
			},

			expectedUpdateStatusCalls: 1,
			expectedCallbacks:         1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			counter := 0
			callbackSent := make(chan struct{})

			metamorphStore := &mocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					return &store.StoreData{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error {
					return nil
				},
				UpdateStatusBulkFunc: func(ctx context.Context, updates []store.UpdateStatus) ([]*store.StoreData, error) {
					if len(tc.updateResp) > 0 {
						counter++
						return tc.updateResp[counter-1], tc.updateErr
					}
					return nil, tc.updateErr
				},
			}

			pm := &mocks.PeerManagerMock{}
			httpClientMock := &mocks.HttpClientMock{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					callbackSent <- struct{}{}
					return &http.Response{
						Body:       readCloser{},
						StatusCode: 200,
					}, nil
				}}

			processor, err := metamorph.NewProcessor(
				metamorphStore,
				pm,
				metamorph.WithNow(func() time.Time { return time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC) }),
				metamorph.WithProcessStatusUpdatesInterval(50*time.Millisecond),
				metamorph.WithProcessStatusUpdatesBatchSize(3),
				metamorph.WithHttpClient(httpClientMock),
			)
			require.NoError(t, err)
			assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

			for _, testInput := range tc.inputs {
				if testInput.txResponseHash != nil {
					processor.ProcessorResponseMap.Set(testInput.txResponseHash, testInput.txResponseHashValue)
				}

				sendErr := processor.SendStatusForTransaction(testInput.hash, testInput.newStatus, "test", testInput.statusErr)
				assert.NoError(t, sendErr)
			}

			callbackCounter := 0
			if tc.expectedCallbacks > 0 {
				select {
				case <-callbackSent:
					callbackCounter++
					if callbackCounter == tc.expectedCallbacks {
						break
					}
				case <-time.NewTimer(time.Second * 5).C:
					t.Fatal("expected callbacks never sent")
				}
			}

			time.Sleep(time.Millisecond * 100)

			assert.Equal(t, tc.expectedUpdateStatusCalls, len(metamorphStore.UpdateStatusBulkCalls()))
			assert.Equal(t, tc.expectedCallbacks, len(httpClientMock.DoCalls()))
			processor.Shutdown()
		})
	}
}

type readCloser struct {
}

func (r readCloser) Read(p []byte) (n int, err error) { return 0, nil }
func (r readCloser) Close() error                     { return nil }

func TestProcessExpiredTransactions(t *testing.T) {
	tt := []struct {
		name        string
		retries     uint32
		minedOrSeen []*store.StoreData

		expectedRequests      int
		expectedAnnouncements int
	}{
		{
			name: "expired txs",

			expectedAnnouncements: 6,
			expectedRequests:      0,
		},
		{
			name:        "expired txs - one seen",
			minedOrSeen: []*store.StoreData{{Hash: testdata.TX1Hash, Status: metamorph_api.Status_SEEN_ON_NETWORK}},

			expectedAnnouncements: 4,
			expectedRequests:      0,
		},
		{
			name:    "expired txs - max retries exceeded",
			retries: 16,

			expectedAnnouncements: 0,
			expectedRequests:      6,
		},
		{
			name:        "expired txs - max retries exceeded, one seen",
			minedOrSeen: []*store.StoreData{{Hash: testdata.TX1Hash, Status: metamorph_api.Status_SEEN_ON_NETWORK}},
			retries:     16,

			expectedAnnouncements: 0,
			expectedRequests:      4,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphStore := &mocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					return &store.StoreData{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error { return nil },
				GetMinedOrSeenFunc: func(ctx context.Context, hashes []*chainhash.Hash) ([]*store.StoreData, error) {
					return tc.minedOrSeen, nil
				},
			}
			pm := &mocks.PeerManagerMock{
				RequestTransactionFunc: func(txHash *chainhash.Hash) p2p.PeerI {
					return nil
				},
				AnnounceTransactionFunc: func(txHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI {
					return nil
				},
			}
			processor, err := metamorph.NewProcessor(metamorphStore, pm,
				metamorph.WithProcessExpiredTxsInterval(time.Millisecond*20),
				metamorph.WithNow(func() time.Time {
					return time.Date(2033, 1, 1, 1, 0, 0, 0, time.UTC)
				}),
			)
			require.NoError(t, err)
			defer processor.Shutdown()

			require.Equal(t, 0, processor.ProcessorResponseMap.Len())

			respSent := processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK)
			respSent.Retries.Add(tc.retries)

			respAnnounced := processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK)
			respAnnounced.Retries.Add(tc.retries)

			respAccepted := processor_response.NewProcessorResponseWithStatus(testdata.TX3Hash, metamorph_api.Status_ACCEPTED_BY_NETWORK)
			respAccepted.Retries.Add(tc.retries)

			processor.ProcessorResponseMap.Set(testdata.TX1Hash, respSent)
			processor.ProcessorResponseMap.Set(testdata.TX2Hash, respAnnounced)
			processor.ProcessorResponseMap.Set(testdata.TX3Hash, respAccepted)

			time.Sleep(50 * time.Millisecond)

			require.Equal(t, tc.expectedAnnouncements, len(pm.AnnounceTransactionCalls()))
			require.Equal(t, tc.expectedRequests, len(pm.RequestTransactionCalls()))
		})
	}
}

func TestProcessorHealth(t *testing.T) {
	tt := []struct {
		name       string
		peersAdded int

		expectedErr error
	}{
		{
			name:       "3 healthy peers",
			peersAdded: 3,
		},
		{
			name:       "1 healthy peer",
			peersAdded: 1,

			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphStore := &mocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					return &store.StoreData{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error { return nil },
			}

			pm := &mocks.PeerManagerMock{
				AddPeerFunc: func(peer p2p.PeerI) error {
					return nil
				},
				GetPeersFunc: func() []p2p.PeerI {
					peers := make([]p2p.PeerI, tc.peersAdded)
					for i := 0; i < tc.peersAdded; i++ {
						peers[i] = &p2p.PeerMock{}
					}

					return peers
				},
			}

			processor, err := metamorph.NewProcessor(metamorphStore, pm,
				metamorph.WithProcessExpiredTxsInterval(time.Millisecond*20),
				metamorph.WithNow(func() time.Time {
					return time.Date(2033, 1, 1, 1, 0, 0, 0, time.UTC)
				}),
			)
			require.NoError(t, err)
			defer processor.Shutdown()

			err = processor.Health()

			require.ErrorIs(t, err, tc.expectedErr)
		})
	}
}
