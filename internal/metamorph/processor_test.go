package metamorph_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/coocood/freecache"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestNewProcessor(t *testing.T) {
	mtmStore := &storeMocks.MetamorphStoreMock{
		GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
			return &store.StoreData{Hash: testdata.TX2Hash}, nil
		},
		SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
	}

	pm := &mocks.PeerManagerMock{ShutdownFunc: func() {}}

	cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))

	tt := []struct {
		name  string
		store store.MetamorphStore
		pm    p2p.PeerManagerI

		expectedError           error
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

			expectedError: metamorph.ErrStoreNil,
		},
		{
			name:  "no pm",
			store: mtmStore,
			pm:    nil,

			expectedError: metamorph.ErrPeerManagerNil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// when
			sut, actualErr := metamorph.NewProcessor(tc.store, cStore, tc.pm, nil,
				metamorph.WithCacheExpiryTime(time.Second*5),
				metamorph.WithProcessorLogger(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: metamorph.LogLevelDefault}))),
			)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, actualErr, tc.expectedError)
				return
			} else {
				require.NoError(t, actualErr)
			}

			defer sut.Shutdown()

			if tc.expectedNonNilProcessor && sut == nil {
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
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{
				SetLockedFunc: func(ctx context.Context, since time.Time, limit int64) error {
					require.Equal(t, int64(5000), limit)
					return tc.setLockedErr
				},
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
			}

			cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))

			pm := &mocks.PeerManagerMock{ShutdownFunc: func() {}}

			// when
			sut, err := metamorph.NewProcessor(metamorphStore, cStore, pm, nil, metamorph.WithLockTxsInterval(20*time.Millisecond))
			require.NoError(t, err)
			defer sut.Shutdown()
			sut.StartLockTransactions()
			time.Sleep(50 * time.Millisecond)

			// then
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
		expectedAnnounceCalls    int
		expectedRequestCalls     int
	}{
		{
			name:            "record not found",
			storeData:       nil,
			storeDataGetErr: store.ErrNotFound,

			expectedResponses: []metamorph_api.Status{
				metamorph_api.Status_STORED,
			},
			expectedResponseMapItems: 0,
			expectedSetCalls:         1,
			expectedAnnounceCalls:    1,
			expectedRequestCalls:     1,
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
			expectedSetCalls:      1,
			expectedAnnounceCalls: 0,
			expectedRequestCalls:  0,
		},
		{
			name:            "store unavailable",
			storeData:       nil,
			storeDataGetErr: sql.ErrConnDone,

			expectedResponses: []metamorph_api.Status{
				metamorph_api.Status_RECEIVED,
			},
			expectedSetCalls:      0,
			expectedAnnounceCalls: 0,
			expectedRequestCalls:  0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			s := &storeMocks.MetamorphStoreMock{
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					require.Equal(t, testdata.TX1Hash[:], key)

					return tc.storeData, tc.storeDataGetErr
				},
				SetFunc: func(ctx context.Context, value *store.StoreData) error {
					require.True(t, bytes.Equal(testdata.TX1Hash[:], value.Hash[:]))

					return nil
				},
				GetUnminedFunc: func(ctx context.Context, since time.Time, limit int64, offset int64) ([]*store.StoreData, error) {
					if offset != 0 {
						return nil, nil
					}
					return []*store.StoreData{
						{
							StoredAt: time.Now(),
							Hash:     testdata.TX1Hash,
							Status:   metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						},
					}, nil
				},
				IncrementRetriesFunc: func(ctx context.Context, hash *chainhash.Hash) error {
					return nil
				},
			}

			cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))

			pm := &mocks.PeerManagerMock{
				AnnounceTransactionFunc: func(txHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI {
					require.True(t, testdata.TX1Hash.IsEqual(txHash))
					return nil
				},
				RequestTransactionFunc: func(txHash *chainhash.Hash) p2p.PeerI { return nil },
			}

			publisher := &mocks.MessageQueueClientMock{
				PublishFunc: func(topic string, hash []byte) error {
					return nil
				},
			}

			sut, err := metamorph.NewProcessor(s, cStore, pm, nil, metamorph.WithMessageQueueClient(publisher))
			require.NoError(t, err)
			require.Equal(t, 0, sut.GetProcessorMapSize())

			responseChannel := make(chan metamorph.StatusAndError)

			// when
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

			sut.ProcessTransaction(context.Background(),
				&metamorph.ProcessorRequest{
					Data: &store.StoreData{
						Hash: testdata.TX1Hash,
					},
					ResponseChannel: responseChannel,
				})
			wg.Wait()

			// then
			require.Equal(t, tc.expectedResponseMapItems, sut.GetProcessorMapSize())
			if tc.expectedResponseMapItems > 0 {
				require.Len(t, pm.AnnounceTransactionCalls(), 1)
			}

			require.Equal(t, tc.expectedSetCalls, len(s.SetCalls()))
			require.Equal(t, tc.expectedAnnounceCalls, len(pm.AnnounceTransactionCalls()))
			require.Equal(t, tc.expectedRequestCalls, len(pm.RequestTransactionCalls()))
		})
	}
}

func TestStartSendStatusForTransaction(t *testing.T) {
	type input struct {
		hash         *chainhash.Hash
		newStatus    metamorph_api.Status
		statusErr    error
		competingTxs []string
	}

	tt := []struct {
		name       string
		inputs     []input
		updateErr  error
		updateResp [][]*store.StoreData

		expectedUpdateStatusCalls int
		expectedDoubleSpendCalls  int
		expectedCallbacks         int
	}{
		{
			name: "new status ANNOUNCED_TO_NETWORK -  updates",
			inputs: []input{
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					statusErr: nil,
				},
			},

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "new status SEEN_ON_NETWORK - updates",
			inputs: []input{
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_SEEN_ON_NETWORK,
					statusErr: nil,
				},
			},

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "new status MINED - update error",
			inputs: []input{
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_MINED,
				},
			},
			updateErr: errors.New("failed to update status"),

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "status update - success",
			inputs: []input{
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					statusErr: nil,
				},
				{
					hash:      testdata.TX2Hash,
					newStatus: metamorph_api.Status_REJECTED,
					statusErr: errors.New("missing inputs"),
				},
				{
					hash:      testdata.TX3Hash,
					newStatus: metamorph_api.Status_SENT_TO_NETWORK,
				},
				{
					hash:      testdata.TX4Hash,
					newStatus: metamorph_api.Status_ACCEPTED_BY_NETWORK,
				},
				{
					hash:      testdata.TX5Hash,
					newStatus: metamorph_api.Status_SEEN_ON_NETWORK,
				},
				{
					hash:         testdata.TX6Hash,
					newStatus:    metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
					competingTxs: []string{"1234"},
				},
			},
			updateResp: [][]*store.StoreData{
				{
					{
						Hash:              testdata.TX1Hash,
						Status:            metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL,
						FullStatusUpdates: true,
						Callbacks:         []store.StoreCallback{{CallbackURL: "http://callback.com"}},
					},
				},
				{
					{
						Hash:              testdata.TX5Hash,
						Status:            metamorph_api.Status_SEEN_ON_NETWORK,
						RejectReason:      "",
						FullStatusUpdates: true,
						Callbacks:         []store.StoreCallback{{CallbackURL: "http://callback.com"}},
					},
				},
				{
					{
						Hash:              testdata.TX6Hash,
						Status:            metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
						FullStatusUpdates: true,
						Callbacks:         []store.StoreCallback{{CallbackURL: "http://callback.com"}},
						CompetingTxs:      []string{"1234"},
					},
				},
			},

			expectedCallbacks:         3,
			expectedUpdateStatusCalls: 2,
			expectedDoubleSpendCalls:  1,
		},
		{
			name: "multiple updates - with duplicates",
			inputs: []input{
				{
					hash:      testdata.TX1Hash,
					newStatus: metamorph_api.Status_REQUESTED_BY_NETWORK,
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
				{
					hash:         testdata.TX2Hash,
					newStatus:    metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
					competingTxs: []string{"1234"},
				},
				{
					hash:         testdata.TX2Hash,
					newStatus:    metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
					competingTxs: []string{"different_competing_tx"},
				},
			},
			updateResp: [][]*store.StoreData{
				{
					{
						Hash:              testdata.TX1Hash,
						Callbacks:         []store.StoreCallback{{CallbackURL: "http://callback.com"}},
						FullStatusUpdates: true,
						Status:            metamorph_api.Status_SEEN_ON_NETWORK,
					},
				},
				{
					{
						Hash:              testdata.TX2Hash,
						Callbacks:         []store.StoreCallback{{CallbackURL: "http://callback.com"}},
						FullStatusUpdates: true,
						Status:            metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
						CompetingTxs:      []string{"1234", "different_competing_tx"},
					},
				},
			},

			expectedUpdateStatusCalls: 1,
			expectedDoubleSpendCalls:  1,
			expectedCallbacks:         2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			counter := 0
			callbackSent := make(chan struct{}, tc.expectedCallbacks)

			metamorphStore := &storeMocks.MetamorphStoreMock{
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
				UpdateDoubleSpendFunc: func(ctx context.Context, updates []store.UpdateStatus) ([]*store.StoreData, error) {
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

			cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))

			pm := &mocks.PeerManagerMock{ShutdownFunc: func() {}}

			callbackSender := &mocks.CallbackSenderMock{
				SendCallbackFunc: func(tx *store.StoreData) {
					callbackSent <- struct{}{}
				},
			}

			statusMessageChannel := make(chan *metamorph.PeerTxMessage, 10)

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, pm, statusMessageChannel, metamorph.WithNow(func() time.Time { return time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC) }), metamorph.WithProcessStatusUpdatesInterval(200*time.Millisecond), metamorph.WithProcessStatusUpdatesBatchSize(3), metamorph.WithCallbackSender(callbackSender))
			require.NoError(t, err)

			// when
			sut.StartProcessStatusUpdatesInStorage()
			sut.StartSendStatusUpdate()

			assert.Equal(t, 0, sut.GetProcessorMapSize())
			for _, testInput := range tc.inputs {
				statusMessageChannel <- &metamorph.PeerTxMessage{
					Hash:         testInput.hash,
					Status:       testInput.newStatus,
					Err:          testInput.statusErr,
					CompetingTxs: testInput.competingTxs,
				}
			}

			timeoutTimer := time.NewTimer(time.Second * 5)
			callbackCounter := 0
			if tc.expectedCallbacks > 0 {
				select {
				case <-callbackSent:
					callbackCounter++
					if callbackCounter == tc.expectedCallbacks {
						break
					}
				case <-timeoutTimer.C:
					t.Fatal("expected callbacks never sent")
				}
			}
			time.Sleep(time.Millisecond * 300)

			// then
			assert.Equal(t, tc.expectedUpdateStatusCalls, len(metamorphStore.UpdateStatusBulkCalls()))
			assert.Equal(t, tc.expectedDoubleSpendCalls, len(metamorphStore.UpdateDoubleSpendCalls()))
			assert.Equal(t, tc.expectedCallbacks, len(callbackSender.SendCallbackCalls()))
			sut.Shutdown()
		})
	}
}

func TestStartProcessSubmittedTxs(t *testing.T) {
	tt := []struct {
		name   string
		txReqs []*metamorph_api.TransactionRequest

		expectedSetBulkCalls     int
		expectedAnnouncedTxCalls int
	}{
		{
			name: "2 submitted txs",
			txReqs: []*metamorph_api.TransactionRequest{
				{
					CallbackUrl:   "callback-1.example.com",
					CallbackToken: "token-1",
					RawTx:         testdata.TX1Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
					MaxTimeout:    10,
				},
				{
					CallbackUrl:   "callback-2.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
					MaxTimeout:    10,
				},
			},

			expectedSetBulkCalls:     1,
			expectedAnnouncedTxCalls: 2,
		},
		{
			name: "5 submitted txs",
			txReqs: []*metamorph_api.TransactionRequest{
				{
					CallbackUrl:   "callback-1.example.com",
					CallbackToken: "token-1",
					RawTx:         testdata.TX1Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
					MaxTimeout:    10,
				},
				{
					CallbackUrl:   "callback-2.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
					MaxTimeout:    10,
				},
				{
					CallbackUrl:   "callback-3.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
					MaxTimeout:    10,
				},
				{
					CallbackUrl:   "callback-4.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
					MaxTimeout:    10,
				},
				{
					CallbackUrl:   "callback-5.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
					MaxTimeout:    10,
				},
			},

			expectedSetBulkCalls:     2,
			expectedAnnouncedTxCalls: 5,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			wg := &sync.WaitGroup{}

			updateBulkCounter := 0
			s := &storeMocks.MetamorphStoreMock{
				SetBulkFunc: func(ctx context.Context, data []*store.StoreData) error {
					return nil
				},
				UpdateStatusBulkFunc: func(ctx context.Context, updates []store.UpdateStatus) ([]*store.StoreData, error) {
					for _, u := range updates {
						require.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, u.Status)
					}
					updateBulkCounter++

					if updateBulkCounter >= tc.expectedSetBulkCalls {
						wg.Done()
					}
					return nil, nil
				},
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
			}
			cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))
			counter := 0
			pm := &mocks.PeerManagerMock{
				AnnounceTransactionFunc: func(txHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI {
					switch counter {
					case 0:
						require.True(t, testdata.TX1Hash.IsEqual(txHash))
					default:
						require.True(t, testdata.TX6Hash.IsEqual(txHash))
					}
					counter++
					return []p2p.PeerI{&mocks.PeerIMock{}}
				},
				ShutdownFunc: func() {},
			}

			publisher := &mocks.MessageQueueClientMock{
				PublishFunc: func(topic string, hash []byte) error {
					return nil
				},
			}
			const submittedTxsBuffer = 5
			submittedTxsChan := make(chan *metamorph_api.TransactionRequest, submittedTxsBuffer)
			sut, err := metamorph.NewProcessor(s, cStore, pm, nil,
				metamorph.WithMessageQueueClient(publisher),
				metamorph.WithSubmittedTxsChan(submittedTxsChan),
				metamorph.WithProcessStatusUpdatesInterval(20*time.Millisecond),
				metamorph.WithProcessTransactionsInterval(20*time.Millisecond),
				metamorph.WithProcessTransactionsBatchSize(4),
			)
			require.NoError(t, err)
			require.Equal(t, 0, sut.GetProcessorMapSize())

			// when
			sut.StartProcessSubmittedTxs()
			sut.StartProcessStatusUpdatesInStorage()
			defer sut.Shutdown()
			wg.Add(1)

			for _, req := range tc.txReqs {
				submittedTxsChan <- req
			}

			c := make(chan struct{})
			go func() {
				wg.Wait()
				c <- struct{}{}
			}()

			select {
			case <-time.NewTimer(2 * time.Second).C:
				t.Fatal("submitted txs have not been stored within 2s")
			case <-c:
			}

			// then
			require.Equal(t, tc.expectedSetBulkCalls, len(s.SetBulkCalls()))
			require.Equal(t, tc.expectedAnnouncedTxCalls, len(pm.AnnounceTransactionCalls()))
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
			// given
			retries := tc.retries

			metamorphStore := &storeMocks.MetamorphStoreMock{
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
							StoredAt: time.Now(),
							Hash:     testdata.TX4Hash,
							Status:   metamorph_api.Status_ANNOUNCED_TO_NETWORK,
							Retries:  retries,
						},
						{
							StoredAt: time.Now(),
							Hash:     testdata.TX5Hash,
							Status:   metamorph_api.Status_STORED,
							Retries:  retries,
						},
					}

					retries++

					return unminedData, tc.getUnminedErr
				},
				IncrementRetriesFunc: func(ctx context.Context, hash *chainhash.Hash) error {
					return nil
				},
			}

			cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))

			pm := &mocks.PeerManagerMock{
				RequestTransactionFunc: func(txHash *chainhash.Hash) p2p.PeerI {
					return nil
				},
				AnnounceTransactionFunc: func(txHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI {
					return nil
				},
				ShutdownFunc: func() {},
			}

			publisher := &mocks.MessageQueueClientMock{
				PublishFunc: func(topic string, hash []byte) error {
					return nil
				},
			}

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, pm, nil,
				metamorph.WithMessageQueueClient(publisher),
				metamorph.WithProcessExpiredTxsInterval(time.Millisecond*20),
				metamorph.WithMaxRetries(10),
				metamorph.WithNow(func() time.Time {
					return time.Date(2033, 1, 1, 1, 0, 0, 0, time.UTC)
				}))
			require.NoError(t, err)
			defer sut.Shutdown()

			// when
			sut.StartProcessExpiredTransactions()

			require.Equal(t, 0, sut.GetProcessorMapSize())

			time.Sleep(50 * time.Millisecond)

			// then
			require.Equal(t, tc.expectedAnnouncements, len(pm.AnnounceTransactionCalls()))
			require.Equal(t, tc.expectedRequests, len(pm.RequestTransactionCalls()))
		})
	}
}

func TestStartProcessMinedCallbacks(t *testing.T) {
	tt := []struct {
		name                  string
		retries               int
		updateMinedErr        error
		processMinedBatchSize int
		processMinedInterval  time.Duration

		expectedTxsBlocks         int
		expectedSendCallbackCalls int
	}{
		{
			name:                  "success - batch size reached",
			processMinedBatchSize: 3,
			processMinedInterval:  20 * time.Second,

			expectedTxsBlocks:         3,
			expectedSendCallbackCalls: 2,
		},
		{
			name:                  "success - interval reached",
			processMinedBatchSize: 50,
			processMinedInterval:  20 * time.Millisecond,

			expectedTxsBlocks:         4,
			expectedSendCallbackCalls: 2,
		},
		{
			name:                  "error - updated mined",
			updateMinedErr:        errors.New("update failed"),
			processMinedBatchSize: 50,
			processMinedInterval:  20 * time.Second,

			expectedSendCallbackCalls: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{
				UpdateMinedFunc: func(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) ([]*store.StoreData, error) {
					require.Len(t, txsBlocks, tc.expectedTxsBlocks)

					return []*store.StoreData{
						{Callbacks: []store.StoreCallback{{CallbackURL: "http://callback.com"}}},
						{Callbacks: []store.StoreCallback{{CallbackURL: "http://callback.com"}}},
						{},
					}, tc.updateMinedErr
				},
				SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) { return 0, nil },
			}
			pm := &mocks.PeerManagerMock{ShutdownFunc: func() {}}
			minedTxsChan := make(chan *blocktx_api.TransactionBlock, 5)
			callbackSender := &mocks.CallbackSenderMock{
				SendCallbackFunc: func(tx *store.StoreData) {},
			}
			cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))
			sut, err := metamorph.NewProcessor(
				metamorphStore,
				cStore,
				pm,
				nil,
				metamorph.WithMinedTxsChan(minedTxsChan),
				metamorph.WithCallbackSender(callbackSender),
				metamorph.WithProcessMinedBatchSize(tc.processMinedBatchSize),
				metamorph.WithProcessMinedInterval(tc.processMinedInterval),
			)
			require.NoError(t, err)

			minedTxsChan <- &blocktx_api.TransactionBlock{}
			minedTxsChan <- &blocktx_api.TransactionBlock{}
			minedTxsChan <- &blocktx_api.TransactionBlock{}
			minedTxsChan <- &blocktx_api.TransactionBlock{}
			minedTxsChan <- nil

			// when
			sut.StartProcessMinedCallbacks()

			time.Sleep(50 * time.Millisecond)
			sut.Shutdown()

			// then
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

			expectedErr: metamorph.ErrUnhealthy,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{
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
							StoredAt: time.Now(),
							Hash:     testdata.TX1Hash,
							Status:   metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						},
						{
							StoredAt: time.Now(),
							Hash:     testdata.TX2Hash,
							Status:   metamorph_api.Status_STORED,
						},
					}, nil
				},
			}

			cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))

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
				ShutdownFunc: func() {},
			}

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, pm, nil,
				metamorph.WithProcessExpiredTxsInterval(time.Millisecond*20),
				metamorph.WithNow(func() time.Time {
					return time.Date(2033, 1, 1, 1, 0, 0, 0, time.UTC)
				}),
			)
			require.NoError(t, err)
			defer sut.Shutdown()

			// when
			actualError := sut.Health()

			// then
			require.ErrorIs(t, actualError, tc.expectedErr)
		})
	}
}

func TestStart(t *testing.T) {
	tt := []struct {
		name     string
		topic    string
		topicErr map[string]error

		expectedError error
	}{
		{
			name: "success",
		},
		{
			name:     "error - subscribe mined txs",
			topic:    metamorph.MinedTxsTopic,
			topicErr: map[string]error{metamorph.MinedTxsTopic: errors.New("failed to subscribe")},

			expectedError: metamorph.ErrFailedToSubscribe,
		},
		{
			name:     "error - subscribe submit txs",
			topic:    metamorph.SubmitTxTopic,
			topicErr: map[string]error{metamorph.SubmitTxTopic: errors.New("failed to subscribe")},

			expectedError: metamorph.ErrFailedToSubscribe,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) {
				return 0, nil
			}}

			cStore := cache.NewFreecacheStore(freecache.NewCache(baseCacheSize))

			pm := &mocks.PeerManagerMock{ShutdownFunc: func() {}}

			var subscribeMinedTxsFunction func([]byte) error
			var subscribeSubmitTxsFunction func([]byte) error
			mqClient := &mocks.MessageQueueClientMock{
				SubscribeFunc: func(topic string, msgFunc func([]byte) error) error {
					switch topic {
					case metamorph.MinedTxsTopic:
						subscribeMinedTxsFunction = msgFunc
					case metamorph.SubmitTxTopic:
						subscribeSubmitTxsFunction = msgFunc
					}

					err, ok := tc.topicErr[topic]
					if ok {
						return err
					}
					return nil
				},
			}

			submittedTxsChan := make(chan *metamorph_api.TransactionRequest, 2)
			minedTxsChan := make(chan *blocktx_api.TransactionBlock, 2)

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, pm, nil,
				metamorph.WithMessageQueueClient(mqClient),
				metamorph.WithSubmittedTxsChan(submittedTxsChan),
				metamorph.WithMinedTxsChan(minedTxsChan),
			)
			require.NoError(t, err)

			// when
			actualError := sut.Start()

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				require.ErrorContains(t, actualError, tc.topic)
				return
			} else {
				require.NoError(t, actualError)
			}

			txBlock := &blocktx_api.TransactionBlock{}
			data, err := proto.Marshal(txBlock)
			require.NoError(t, err)

			_ = subscribeMinedTxsFunction([]byte("invalid data"))
			_ = subscribeMinedTxsFunction(data)

			txRequest := &metamorph_api.TransactionRequest{}
			data, err = proto.Marshal(txRequest)
			require.NoError(t, err)
			_ = subscribeSubmitTxsFunction([]byte("invalid data"))
			_ = subscribeSubmitTxsFunction(data)

			sut.Shutdown()
		})
	}
}
