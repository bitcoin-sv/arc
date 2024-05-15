package metamorph_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:generate moq -pkg mocks -out ./mocks/store_mock.go ./store/ MetamorphStore
//go:generate moq -pkg mocks -out ./mocks/message_queue_mock.go . MessageQueueClient
//go:generate moq -pkg mocks -out ./mocks/callback_sender_mock.go . CallbackSender
//go:generate moq -pkg mocks -out ./mocks/peer_manager_mock.go . PeerManager

func TestNewProcessor(t *testing.T) {
	mtmStore := &mocks.MetamorphStoreMock{
		GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
			return &store.StoreData{Hash: testdata.TX2Hash}, nil
		},
		SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
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

func TestStartLockTransactions(t *testing.T) {
	tt := []struct {
		name         string
		setLockedErr error

		expectedSetLockedCalls int
	}{
		{
			name: "no error",

			expectedSetLockedCalls: 2,
		},
		{
			name:         "error",
			setLockedErr: errors.New("failed to set locked"),

			expectedSetLockedCalls: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphStore := &mocks.MetamorphStoreMock{
				SetLockedFunc: func(ctx context.Context, since time.Time, limit int64) error {
					require.Equal(t, int64(5000), limit)
					return tc.setLockedErr
				},
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
			}

			pm := &mocks.PeerManagerMock{}

			processor, err := metamorph.NewProcessor(
				metamorphStore,
				pm,
				metamorph.WithLockTxsInterval(20*time.Millisecond),
			)
			require.NoError(t, err)
			defer processor.Shutdown()
			processor.StartLockTransactions()
			time.Sleep(50 * time.Millisecond)

			require.Equal(t, tc.expectedSetLockedCalls, len(metamorphStore.SetLockedCalls()))
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
			expectedSetCalls: 1,
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
				GetUnminedFunc: func(ctx context.Context, since time.Time, limit int64, offset int64) ([]*store.StoreData, error) {
					if offset != 0 {
						return nil, nil
					}
					return []*store.StoreData{
						{
							StoredAt:    time.Now(),
							AnnouncedAt: time.Now(),
							Hash:        testdata.TX1Hash,
							Status:      metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						},
					}, nil
				},
				IncrementRetriesFunc: func(ctx context.Context, hash *chainhash.Hash) error {
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

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "tx in response map - current status REJECTED, new status SEEN_ON_NETWORK - still updates",
			inputs: []input{
				{
					hash:                testdata.TX1Hash,
					newStatus:           metamorph_api.Status_SEEN_ON_NETWORK,
					statusErr:           nil,
					txResponseHash:      testdata.TX1Hash,
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX1Hash),
				},
			},

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "new status MINED - update error",
			inputs: []input{
				{
					hash:                testdata.TX1Hash,
					newStatus:           metamorph_api.Status_MINED,
					txResponseHash:      testdata.TX1Hash,
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX1Hash),
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
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX1Hash),
				},
				{
					hash:                testdata.TX2Hash,
					newStatus:           metamorph_api.Status_REJECTED,
					statusErr:           errors.New("missing inputs"),
					txResponseHash:      testdata.TX2Hash,
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX2Hash),
				},
				{
					hash:                testdata.TX3Hash,
					newStatus:           metamorph_api.Status_SENT_TO_NETWORK,
					txResponseHash:      testdata.TX3Hash,
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX3Hash),
				},
				{
					hash:                testdata.TX4Hash,
					newStatus:           metamorph_api.Status_ACCEPTED_BY_NETWORK,
					txResponseHash:      testdata.TX4Hash,
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX4Hash),
				},
				{
					hash:                testdata.TX5Hash,
					newStatus:           metamorph_api.Status_SEEN_ON_NETWORK,
					txResponseHash:      testdata.TX5Hash,
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX5Hash),
				},
				{
					hash:                testdata.TX6Hash,
					newStatus:           metamorph_api.Status_REQUESTED_BY_NETWORK,
					txResponseHash:      testdata.TX6Hash,
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX6Hash),
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
					txResponseHashValue: processor_response.NewProcessorResponse(testdata.TX1Hash),
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
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
				UpdateStatusBulkFunc: func(ctx context.Context, updates []store.UpdateStatus) ([]*store.StoreData, error) {
					if len(tc.updateResp) > 0 {
						counter++
						return tc.updateResp[counter-1], tc.updateErr
					}
					return nil, tc.updateErr
				},
				IncrementRetriesFunc: func(ctx context.Context, hash *chainhash.Hash) error {
					return nil
				},
			}

			pm := &mocks.PeerManagerMock{}

			callbackSender := &mocks.CallbackSenderMock{
				SendCallbackFunc: func(logger *slog.Logger, tx *store.StoreData) {
					callbackSent <- struct{}{}
				},
			}

			processor, err := metamorph.NewProcessor(
				metamorphStore,
				pm,
				metamorph.WithNow(func() time.Time { return time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC) }),
				metamorph.WithProcessStatusUpdatesInterval(50*time.Millisecond),
				metamorph.WithProcessStatusUpdatesBatchSize(3),
				metamorph.WithCallbackSender(callbackSender),
			)
			require.NoError(t, err)

			processor.StartProcessStatusUpdatesInStorage()

			assert.Equal(t, 0, processor.ProcessorResponseMap.Len())
			for _, testInput := range tc.inputs {
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
			assert.Equal(t, tc.expectedCallbacks, len(callbackSender.SendCallbackCalls()))
			processor.Shutdown()
		})
	}
}

func TestProcessExpiredTransactions(t *testing.T) {
	tt := []struct {
		name          string
		retries       int
		getUnminedErr error

		expectedRequests      int
		expectedAnnouncements int
	}{
		{
			name:    "expired txs",
			retries: 4,

			expectedAnnouncements: 2,
			expectedRequests:      2,
		},
		{
			name:    "expired txs - max retries exceeded",
			retries: 16,

			expectedAnnouncements: 0,
			expectedRequests:      0,
		},
		{
			name:          "error - get unmined",
			retries:       4,
			getUnminedErr: errors.New("failed to get unmined"),

			expectedAnnouncements: 0,
			expectedRequests:      0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			retries := tc.retries

			metamorphStore := &mocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					return &store.StoreData{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
				GetUnminedFunc: func(ctx context.Context, since time.Time, limit int64, offset int64) ([]*store.StoreData, error) {
					if offset != 0 {
						return nil, nil
					}
					unminedData := []*store.StoreData{
						{
							StoredAt:    time.Now(),
							AnnouncedAt: time.Now(),
							Hash:        testdata.TX4Hash,
							Status:      metamorph_api.Status_ANNOUNCED_TO_NETWORK,
							Retries:     retries,
						},
						{
							StoredAt:    time.Now(),
							AnnouncedAt: time.Now(),
							Hash:        testdata.TX5Hash,
							Status:      metamorph_api.Status_STORED,
							Retries:     retries,
						},
					}

					retries++

					return unminedData, tc.getUnminedErr
				},
				IncrementRetriesFunc: func(ctx context.Context, hash *chainhash.Hash) error {
					return nil
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

			publisher := &mocks.MessageQueueClientMock{
				PublishRegisterTxsFunc: func(hash []byte) error {
					return nil
				},
				PublishRequestTxFunc: func(hash []byte) error {
					return nil
				},
			}

			processor, err := metamorph.NewProcessor(metamorphStore, pm,
				metamorph.WithMessageQueueClient(publisher),
				metamorph.WithProcessExpiredTxsInterval(time.Millisecond*20),
				metamorph.WithMaxRetries(10),
				metamorph.WithNow(func() time.Time {
					return time.Date(2033, 1, 1, 1, 0, 0, 0, time.UTC)
				}),
			)
			require.NoError(t, err)
			defer processor.Shutdown()

			processor.StartProcessExpiredTransactions()

			require.Equal(t, 0, processor.ProcessorResponseMap.Len())

			// some dummy txs in map shouldn't affect announcements
			processor.ProcessorResponseMap.Set(testdata.TX1Hash, processor_response.NewProcessorResponse(testdata.TX1Hash))
			processor.ProcessorResponseMap.Set(testdata.TX2Hash, processor_response.NewProcessorResponse(testdata.TX2Hash))
			processor.ProcessorResponseMap.Set(testdata.TX3Hash, processor_response.NewProcessorResponse(testdata.TX3Hash))

			time.Sleep(50 * time.Millisecond)

			require.Equal(t, tc.expectedAnnouncements, len(pm.AnnounceTransactionCalls()))
			require.Equal(t, tc.expectedRequests, len(pm.RequestTransactionCalls()))
		})
	}
}

func TestStartProcessMinedCallbacks(t *testing.T) {
	tt := []struct {
		name           string
		retries        int
		updateMinedErr error
		panic          bool

		expectedSendCallbackCalls int
	}{
		{
			name:                      "success",
			expectedSendCallbackCalls: 2,
		},
		{
			name:           "error - updated mined",
			updateMinedErr: errors.New("update failed"),

			expectedSendCallbackCalls: 0,
		},
		{
			name:  "panic",
			panic: true,

			expectedSendCallbackCalls: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			metamorphStore := &mocks.MetamorphStoreMock{
				UpdateMinedFunc: func(ctx context.Context, txsBlocks *blocktx_api.TransactionBlocks) ([]*store.StoreData, error) {
					if tc.panic {
						panic("panic in updated mined function")
					}
					return []*store.StoreData{{CallbackUrl: "http://callback.com"}, {CallbackUrl: "http://callback.com"}, {}}, tc.updateMinedErr
				},
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
			}
			pm := &mocks.PeerManagerMock{}
			minedTxsChan := make(chan *blocktx_api.TransactionBlocks, 5)
			callbackSender := &mocks.CallbackSenderMock{
				SendCallbackFunc: func(logger *slog.Logger, tx *store.StoreData) {},
			}
			processor, err := metamorph.NewProcessor(
				metamorphStore,
				pm,
				metamorph.WithMinedTxsChan(minedTxsChan),
				metamorph.WithCallbackSender(callbackSender),
			)
			require.NoError(t, err)

			minedTxsChan <- &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{{}, {}, {}}}
			minedTxsChan <- nil

			processor.StartProcessMinedCallbacks()

			time.Sleep(50 * time.Millisecond)
			processor.Shutdown()

			require.Equal(t, tc.expectedSendCallbackCalls, len(callbackSender.SendCallbackCalls()))
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
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
				GetUnminedFunc: func(ctx context.Context, since time.Time, limit int64, offset int64) ([]*store.StoreData, error) {
					if offset != 0 {
						return nil, nil
					}
					return []*store.StoreData{
						{
							StoredAt:    time.Now(),
							AnnouncedAt: time.Now(),
							Hash:        testdata.TX1Hash,
							Status:      metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						},
						{
							StoredAt:    time.Now(),
							AnnouncedAt: time.Now(),
							Hash:        testdata.TX2Hash,
							Status:      metamorph_api.Status_STORED,
						},
					}, nil
				},
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
