package metamorph_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ccoveille/go-safecast"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	btxMocks "github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/cache"
	cacheMocks "github.com/bitcoin-sv/arc/internal/cache/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
	"github.com/bitcoin-sv/arc/internal/mq"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
)

func TestNewProcessor(t *testing.T) {
	mtmStore := &storeMocks.MetamorphStoreMock{
		GetFunc: func(_ context.Context, _ []byte) (*store.Data, error) {
			return &store.Data{Hash: testdata.TX2Hash}, nil
		},
		SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
	}

	nMessenger := &mocks.MediatorMock{}
	cStore := &cacheMocks.StoreMock{}

	tt := []struct {
		name      string
		store     store.MetamorphStore
		messenger metamorph.Mediator

		expectedError           error
		expectedNonNilProcessor bool
	}{
		{
			name:                    "success",
			store:                   mtmStore,
			messenger:               nMessenger,
			expectedNonNilProcessor: true,
		},
		{
			name:      "no store",
			store:     nil,
			messenger: nMessenger,

			expectedError: metamorph.ErrStoreNil,
		},
		{
			name:      "no pm",
			store:     mtmStore,
			messenger: nil,

			expectedError: metamorph.ErrPeerMessengerNil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// when
			sut, actualErr := metamorph.NewProcessor(tc.store, cStore, tc.messenger, nil,
				metamorph.WithReBroadcastExpiration(time.Second*5),
				metamorph.WithProcessorLogger(slog.Default()),
			)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, actualErr, tc.expectedError)
				return
			}
			require.NoError(t, actualErr)

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

		expectedSetLockedCallsGreaterThan int
	}{
		{
			name: "no error",

			expectedSetLockedCallsGreaterThan: 1,
		},
		{
			name:         "error",
			setLockedErr: errors.New("failed to set locked"),

			expectedSetLockedCallsGreaterThan: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{
				SetLockedFunc: func(_ context.Context, _ time.Time, limit int64) error {
					require.Equal(t, int64(50), limit)
					return tc.setLockedErr
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}

			messenger := &mocks.MediatorMock{}
			cStore := &cacheMocks.StoreMock{}

			// when
			sut, err := metamorph.NewProcessor(metamorphStore, cStore, messenger, nil, metamorph.WithLockTxsInterval(20*time.Millisecond))
			require.NoError(t, err)
			defer sut.Shutdown()

			sut.StartLockTransactions()
			time.Sleep(60 * time.Millisecond)

			// then
			require.GreaterOrEqual(t, len(metamorphStore.SetLockedCalls()), tc.expectedSetLockedCallsGreaterThan)
		})
	}
}

func TestProcessTransaction(t *testing.T) {
	tt := []struct {
		name            string
		storeData       *store.Data
		storeDataGetErr error
		registerTxErr   error

		expectedResponses     []metamorph_api.Status
		expectedSetCalls      int
		expectedAnnounceCalls int
		expectedRequestCalls  int
		expectedPublishCalls  int
	}{
		{
			name:            "record not found - success",
			storeData:       nil,
			storeDataGetErr: store.ErrNotFound,

			expectedResponses: []metamorph_api.Status{
				metamorph_api.Status_STORED,
				metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
			expectedSetCalls:      1,
			expectedAnnounceCalls: 1,
			expectedRequestCalls:  1,
		},
		{
			name:            "record not found - register tx with blocktx client failed",
			storeData:       nil,
			storeDataGetErr: store.ErrNotFound,
			registerTxErr:   errors.New("failed to register tx"),

			expectedResponses: []metamorph_api.Status{
				metamorph_api.Status_STORED,
				metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
			expectedSetCalls:      1,
			expectedAnnounceCalls: 1,
			expectedRequestCalls:  1,
			expectedPublishCalls:  1,
		},
		{
			name: "record found",
			storeData: &store.Data{
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
				GetFunc: func(_ context.Context, key []byte) (*store.Data, error) {
					require.Equal(t, testdata.TX1Hash[:], key)

					return tc.storeData, tc.storeDataGetErr
				},
				SetFunc: func(_ context.Context, value *store.Data) error {
					require.True(t, bytes.Equal(testdata.TX1Hash[:], value.Hash[:]))

					return nil
				},
				GetUnseenFunc: func(_ context.Context, _ time.Time, _ int64, offset int64) ([]*store.Data, error) {
					if offset != 0 {
						return nil, nil
					}
					return []*store.Data{
						{
							StoredAt: time.Now(),
							Hash:     testdata.TX1Hash,
							Status:   metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						},
					}, nil
				},
				IncrementRetriesFunc: func(_ context.Context, _ *chainhash.Hash) error {
					return nil
				},
			}

			cStore := &cacheMocks.StoreMock{
				SetFunc: func(_ string, _ []byte, _ time.Duration) error {
					return nil
				},
			}

			messenger := &mocks.MediatorMock{
				AskForTxAsyncFunc:   func(_ context.Context, _ *chainhash.Hash) {},
				AnnounceTxAsyncFunc: func(_ context.Context, _ *chainhash.Hash, _ []byte) {},
			}

			publisher := &mqMocks.MessageQueueClientMock{
				PublishAsyncFunc: func(_ string, _ []byte) error {
					return nil
				},
			}

			blocktxClient := &btxMocks.ClientMock{RegisterTransactionFunc: func(_ context.Context, _ []byte) error { return tc.registerTxErr }}

			sut, err := metamorph.NewProcessor(s, cStore, messenger, nil, metamorph.WithMessageQueueClient(publisher), metamorph.WithBlocktxClient(blocktxClient))
			require.NoError(t, err)
			require.Equal(t, 0, sut.GetProcessorMapSize())

			responseChannel := make(chan metamorph.StatusAndError)

			// when
			var wg sync.WaitGroup
			wg.Add(len(tc.expectedResponses)) // nolint:revive // intentional
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
					Data: &store.Data{
						Hash: testdata.TX1Hash,
					},
					ResponseChannel: responseChannel,
				})
			wg.Wait()
			time.Sleep(250 * time.Millisecond)
			// then
			require.Equal(t, tc.expectedSetCalls, len(s.SetCalls()))
			require.Equal(t, tc.expectedAnnounceCalls, len(messenger.AnnounceTxAsyncCalls()))
			require.Equal(t, tc.expectedRequestCalls, len(messenger.AskForTxAsyncCalls()))
			require.Equal(t, tc.expectedPublishCalls, len(publisher.PublishAsyncCalls()))
		})
	}
}

func TestStartSendStatusForTransaction(t *testing.T) {
	type input struct {
		hash         *chainhash.Hash
		newStatus    metamorph_api.Status
		statusErr    error
		competingTxs []string
		registered   bool
	}

	tt := []struct {
		name       string
		inputs     []input
		updateErr  error
		updateResp [][]*store.Data

		expectedUpdateStatusCalls int
		expectedDoubleSpendCalls  int
		expectedCallbacks         int
	}{
		{
			name: "new status ANNOUNCED_TO_NETWORK -  updates",
			inputs: []input{
				{
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					statusErr:  nil,
					registered: true,
				},
			},

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "new status SEEN_ON_NETWORK - updates",
			inputs: []input{
				{
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_SEEN_ON_NETWORK,
					statusErr:  nil,
					registered: true,
				},
			},

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "new status MINED - update error",
			inputs: []input{
				{
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_MINED,
					registered: true,
				},
			},
			updateErr: errors.New("failed to update status"),

			expectedUpdateStatusCalls: 1,
		},
		{
			name: "status update - success",
			inputs: []input{
				{
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					statusErr:  nil,
					registered: true,
				},
				{
					hash:       testdata.TX2Hash,
					newStatus:  metamorph_api.Status_REJECTED,
					statusErr:  errors.New("missing inputs"),
					registered: true,
				},
				{
					hash:       testdata.TX3Hash,
					newStatus:  metamorph_api.Status_SENT_TO_NETWORK,
					registered: true,
				},
				{
					hash:       testdata.TX4Hash,
					newStatus:  metamorph_api.Status_ACCEPTED_BY_NETWORK,
					registered: true,
				},
				{
					hash:       testdata.TX5Hash,
					newStatus:  metamorph_api.Status_SEEN_ON_NETWORK,
					registered: true,
				},
				{
					hash:         testdata.TX6Hash,
					newStatus:    metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
					competingTxs: []string{"1234"},
					registered:   true,
				},
			},
			updateResp: [][]*store.Data{
				{
					{
						Hash:              testdata.TX1Hash,
						Status:            metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL,
						FullStatusUpdates: true,
						Callbacks:         []store.Callback{{CallbackURL: "http://callback.com"}},
					},
				},
				{
					{
						Hash:              testdata.TX5Hash,
						Status:            metamorph_api.Status_SEEN_ON_NETWORK,
						RejectReason:      "",
						FullStatusUpdates: true,
						Callbacks:         []store.Callback{{CallbackURL: "http://callback.com"}},
					},
				},
				{
					{
						Hash:              testdata.TX6Hash,
						Status:            metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
						FullStatusUpdates: true,
						Callbacks:         []store.Callback{{CallbackURL: "http://callback.com"}},
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
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_REQUESTED_BY_NETWORK,
					registered: true,
				},
				{
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_SEEN_ON_NETWORK,
					registered: true,
				},
				{
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_SENT_TO_NETWORK,
					registered: true,
				},
				{
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_ACCEPTED_BY_NETWORK,
					registered: true,
				},
				{
					hash:         testdata.TX2Hash,
					newStatus:    metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
					competingTxs: []string{"1234"},
					registered:   true,
				},
				{
					hash:         testdata.TX2Hash,
					newStatus:    metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
					competingTxs: []string{"different_competing_tx"},
					registered:   true,
				},
			},
			updateResp: [][]*store.Data{
				{
					{
						Hash:              testdata.TX1Hash,
						Callbacks:         []store.Callback{{CallbackURL: "http://callback.com"}},
						FullStatusUpdates: true,
						Status:            metamorph_api.Status_SEEN_ON_NETWORK,
					},
				},
				{
					{
						Hash:              testdata.TX2Hash,
						Callbacks:         []store.Callback{{CallbackURL: "http://callback.com"}},
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
		{
			name: "not registered in cache",
			inputs: []input{
				{
					hash:       testdata.TX1Hash,
					newStatus:  metamorph_api.Status_ANNOUNCED_TO_NETWORK,
					statusErr:  nil,
					registered: false,
				},
			},

			expectedUpdateStatusCalls: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			counter := 0

			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetFunc: func(_ context.Context, _ []byte) (*store.Data, error) {
					return &store.Data{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
				UpdateStatusFunc: func(_ context.Context, _ []store.UpdateStatus) ([]*store.Data, error) {
					if len(tc.updateResp) > 0 {
						counter++
						return tc.updateResp[counter-1], tc.updateErr
					}
					return nil, tc.updateErr
				},
				UpdateDoubleSpendFunc: func(_ context.Context, _ []store.UpdateStatus, _ bool) ([]*store.Data, error) {
					if len(tc.updateResp) > 0 {
						counter++
						return tc.updateResp[counter-1], tc.updateErr
					}
					return nil, tc.updateErr
				},
				IncrementRetriesFunc: func(_ context.Context, _ *chainhash.Hash) error {
					return nil
				},
			}

			messenger := &mocks.MediatorMock{}
			cStore := cache.NewMemoryStore()
			for _, i := range tc.inputs {
				if i.registered {
					err := cStore.Set(i.hash.String(), []byte("1"), 10*time.Minute)
					require.NoError(t, err)
				}
			}

			statusMessageChannel := make(chan *metamorph_p2p.TxStatusMessage, 10)

			mqClient := &mqMocks.MessageQueueClientMock{
				PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error {
					return nil
				},
			}

			sut, err := metamorph.NewProcessor(
				metamorphStore,
				cStore,
				messenger,
				statusMessageChannel,
				metamorph.WithNow(func() time.Time { return time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC) }),
				metamorph.WithStatusUpdatesInterval(200*time.Millisecond),
				metamorph.WithProcessStatusUpdatesBatchSize(3),
				metamorph.WithMessageQueueClient(mqClient),
			)
			require.NoError(t, err)

			// when
			sut.StartProcessStatusUpdatesInStorage()
			sut.StartSendStatusUpdate()

			require.Equal(t, 0, sut.GetProcessorMapSize())
			for _, testInput := range tc.inputs {
				statusMessageChannel <- &metamorph_p2p.TxStatusMessage{
					Hash:         testInput.hash,
					Status:       testInput.newStatus,
					Err:          testInput.statusErr,
					CompetingTxs: testInput.competingTxs,
				}
			}

			time.Sleep(time.Millisecond * 300)

			// then
			require.Equal(t, tc.expectedUpdateStatusCalls, len(metamorphStore.UpdateStatusCalls()))
			require.Equal(t, tc.expectedDoubleSpendCalls, len(metamorphStore.UpdateDoubleSpendCalls()))
			require.Equal(t, tc.expectedCallbacks, len(mqClient.PublishMarshalCalls()))
			sut.Shutdown()
		})
	}
}

func TestStartProcessSubmitted(t *testing.T) {
	tt := []struct {
		name   string
		txReqs []*metamorph_api.PostTransactionRequest

		expectedSetBulkCalls  int32
		expectedAnnounceCalls int32
		expcetedUpdates       int
	}{
		{
			name: "2 submitted txs",
			txReqs: []*metamorph_api.PostTransactionRequest{
				{
					CallbackUrl:   "callback-1.example.com",
					CallbackToken: "token-1",
					RawTx:         testdata.TX1Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
				},
				{
					CallbackUrl:   "callback-2.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
				},
			},

			expectedSetBulkCalls:  1,
			expectedAnnounceCalls: 2,
			expcetedUpdates:       2,
		},
		{
			name: "5 submitted txs",
			txReqs: []*metamorph_api.PostTransactionRequest{
				{
					CallbackUrl:   "callback-1.example.com",
					CallbackToken: "token-1",
					RawTx:         testdata.TX1Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
				},
				{
					CallbackUrl:   "callback-2.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
				},
				{
					CallbackUrl:   "callback-3.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
				},
				{
					CallbackUrl:   "callback-4.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
				},
				{
					CallbackUrl:   "callback-5.example.com",
					CallbackToken: "token-2",
					RawTx:         testdata.TX6Raw.Bytes(),
					WaitForStatus: metamorph_api.Status_RECEIVED,
				},
			},

			expectedSetBulkCalls:  2,
			expectedAnnounceCalls: 5,
			expcetedUpdates:       5,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			s := &storeMocks.MetamorphStoreMock{
				SetBulkFunc: func(_ context.Context, _ []*store.Data) error {
					return nil
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}
			stopCh := make(chan struct{}, 5)
			announceMsgCounter := &atomic.Int32{}
			cStore := &cacheMocks.StoreMock{
				SetFunc: func(_ string, _ []byte, _ time.Duration) error {
					return nil
				},
			}
			messenger := &mocks.MediatorMock{
				AnnounceTxAsyncFunc: func(_ context.Context, _ *chainhash.Hash, _ []byte) {
					announceMsgCounter.Add(1)
					if announceMsgCounter.Load() >= tc.expectedAnnounceCalls {
						stopCh <- struct{}{}
					}
				},
			}

			blocktxClient := &btxMocks.ClientMock{RegisterTransactionFunc: func(_ context.Context, _ []byte) error { return nil }}

			const submittedTxsBuffer = 5
			submittedTxsChan := make(chan *metamorph_api.PostTransactionRequest, submittedTxsBuffer)
			sut, err := metamorph.NewProcessor(s, cStore, messenger, nil,
				metamorph.WithSubmittedTxsChan(submittedTxsChan),
				metamorph.WithProcessTransactionsInterval(20*time.Millisecond),
				metamorph.WithProcessTransactionsBatchSize(4),
				metamorph.WithBlocktxClient(blocktxClient),
			)
			require.NoError(t, err)
			require.Equal(t, 0, sut.GetProcessorMapSize())

			// when
			sut.StartProcessSubmitted()
			defer sut.Shutdown()

			for _, req := range tc.txReqs {
				submittedTxsChan <- req
			}

			select {
			case <-time.NewTimer(1 * time.Second).C:
				t.Fatal("submitted txs have not been stored within 2s")
			case <-stopCh:
			}

			// then
			l, err := safecast.ToInt32(len(s.SetBulkCalls()))
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedSetBulkCalls, l)
			assert.Equal(t, tc.expectedAnnounceCalls, announceMsgCounter.Load())
		})
	}
}

func TestReAnnounceUnseen(t *testing.T) {
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

			expectedAnnouncements: 1,
			expectedRequests:      1,
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
				GetFunc: func(_ context.Context, _ []byte) (*store.Data, error) {
					return &store.Data{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
				GetUnseenFunc: func(_ context.Context, _ time.Time, _ int64, offset int64) ([]*store.Data, error) {
					if offset != 0 {
						return nil, nil
					}
					unminedData := []*store.Data{
						{
							StoredAt: time.Now(),
							Hash:     testdata.TX4Hash,
							Status:   metamorph_api.Status_ANNOUNCED_TO_NETWORK,
							Retries:  retries + 1,
						},
						{
							StoredAt: time.Now(),
							Hash:     testdata.TX5Hash,
							Status:   metamorph_api.Status_STORED,
							Retries:  retries,
						},
					}

					return unminedData, tc.getUnminedErr
				},
				IncrementRetriesFunc: func(_ context.Context, _ *chainhash.Hash) error {
					retries++
					return nil
				},
			}

			cStore := &cacheMocks.StoreMock{
				SetFunc: func(_ string, _ []byte, _ time.Duration) error {
					return nil
				},
			}

			messenger := &mocks.MediatorMock{
				AskForTxAsyncFunc:   func(_ context.Context, _ *chainhash.Hash) {},
				AnnounceTxAsyncFunc: func(_ context.Context, _ *chainhash.Hash, _ []byte) {},
			}

			publisher := &mqMocks.MessageQueueClientMock{
				PublishAsyncFunc: func(_ string, _ []byte) error {
					return nil
				},
			}

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, messenger, nil,
				metamorph.WithMessageQueueClient(publisher),
				metamorph.WithMaxRetries(10),
				metamorph.WithNow(func() time.Time {
					return time.Date(2033, 1, 1, 1, 0, 0, 0, time.UTC)
				}))
			require.NoError(t, err)
			defer sut.Shutdown()

			// when
			metamorph.ReAnnounceUnseen(context.TODO(), sut)

			require.Equal(t, 0, sut.GetProcessorMapSize())

			time.Sleep(250 * time.Millisecond)

			// then
			require.Equal(t, tc.expectedAnnouncements, len(messenger.AnnounceTxAsyncCalls()))
			require.Equal(t, tc.expectedRequests, len(messenger.AskForTxAsyncCalls()))
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
				UpdateMinedFunc: func(_ context.Context, txsBlocks []*blocktx_api.TransactionBlock) ([]*store.Data, error) {
					require.Len(t, txsBlocks, tc.expectedTxsBlocks)

					return []*store.Data{
						{Hash: testdata.TX1Hash, Callbacks: []store.Callback{{CallbackURL: "https://callback.com"}}},
						{Hash: testdata.TX2Hash, Callbacks: []store.Callback{{CallbackURL: "https://callback.com"}}},
						{Hash: testdata.TX1Hash},
					}, tc.updateMinedErr
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}
			pm := &mocks.MediatorMock{}
			minedTxsChan := make(chan *blocktx_api.TransactionBlocks, 5)
			callbackSender := &mocks.CallbackSenderMock{
				SendCallbackFunc: func(_ context.Context, _ *store.Data) {},
			}

			mqClient := &mqMocks.MessageQueueClientMock{
				PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error {
					return nil
				},
			}

			cStore := &cacheMocks.StoreMock{
				DelFunc: func(_ ...string) error {
					return nil
				},
			}
			sut, err := metamorph.NewProcessor(
				metamorphStore,
				cStore,
				pm,
				nil,
				metamorph.WithMinedTxsChan(minedTxsChan),
				metamorph.WithCallbackSender(callbackSender),
				metamorph.WithProcessMinedBatchSize(tc.processMinedBatchSize),
				metamorph.WithProcessMinedInterval(tc.processMinedInterval),
				metamorph.WithMessageQueueClient(mqClient),
			)
			require.NoError(t, err)

			minedTxsChan <- &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{{}}}
			minedTxsChan <- &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{{}}}
			minedTxsChan <- &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{{}}}
			minedTxsChan <- &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{{}}}
			minedTxsChan <- nil

			// when
			sut.StartProcessMinedCallbacks()

			time.Sleep(50 * time.Millisecond)
			sut.Shutdown()

			// then
			require.Equal(t, tc.expectedSendCallbackCalls, len(mqClient.PublishMarshalCalls()))
		})
	}
}

func TestProcessDoubleSpendAttemptCallbacks(t *testing.T) {
	// given
	metamorphStore := &storeMocks.MetamorphStoreMock{
		SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
		UpdateStatusFunc: func(_ context.Context, _ []store.UpdateStatus) ([]*store.Data, error) {
			return []*store.Data{
				{
					Hash:   testdata.TX1Hash,
					Status: metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
					Callbacks: []store.Callback{
						{CallbackURL: "http://callback.com"},
					},
					FullStatusUpdates: true,
				}}, nil
		},
		UpdateDoubleSpendFunc: func(_ context.Context, _ []store.UpdateStatus, _ bool) ([]*store.Data, error) {
			return []*store.Data{
				{
					Hash:   testdata.TX1Hash,
					Status: metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
					Callbacks: []store.Callback{
						{CallbackURL: "http://callback.com"},
					},
					FullStatusUpdates: true,
				}}, nil
		},
	}
	pm := &mocks.MediatorMock{}
	callbackSender := &mocks.CallbackSenderMock{
		SendCallbackFunc: func(_ context.Context, _ *store.Data) {},
	}

	mqClient := &mqMocks.MessageQueueClientMock{
		PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error {
			return nil
		},
	}

	i := 0
	cStore := &cacheMocks.StoreMock{
		DelFunc: func(_ ...string) error {
			return nil
		},
		GetFunc: func(_ string) ([]byte, error) {
			return testdata.TX1Hash.CloneBytes(), nil
		},
		MapGetFunc: func(_ string, _ string) ([]byte, error) {
			j, _ := json.Marshal(store.UpdateStatus{
				CompetingTxs: []string{testdata.TX2Hash.String()},
				Hash:         chainhash.Hash(testdata.TX1Hash.CloneBytes()),
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
			})
			return j, nil
		},
		MapSetFunc: func(_ string, _ string, _ []byte) error {
			return nil
		},
		MapLenFunc: func(_ string) (int64, error) {
			return 1, nil
		},
		MapExtractAllFunc: func(_ string) (map[string][]byte, error) {
			i++
			if i > 1 {
				return nil, nil
			}
			j, _ := json.Marshal(store.UpdateStatus{
				CompetingTxs: []string{testdata.TX2Hash.String()},
				Hash:         chainhash.Hash(testdata.TX1Hash.CloneBytes()),
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
			})
			return map[string][]byte{testdata.TX2Hash.String(): j}, nil
		},
	}
	statusMessageChannel := make(chan *metamorph_p2p.TxStatusMessage)
	sut, err := metamorph.NewProcessor(
		metamorphStore,
		cStore,
		pm,
		statusMessageChannel,
		metamorph.WithCallbackSender(callbackSender),
		metamorph.WithStatusUpdatesInterval(10*time.Millisecond),
		metamorph.WithMessageQueueClient(mqClient),
	)
	require.NoError(t, err)
	// when
	sut.StartSendStatusUpdate()
	sut.StartProcessStatusUpdatesInStorage()

	statusMessageChannel <- &metamorph_p2p.TxStatusMessage{
		Hash:         testdata.TX1Hash,
		Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
		Err:          nil,
		CompetingTxs: []string{testdata.TX2Hash.String()},
	}

	time.Sleep(200 * time.Millisecond)
	sut.Shutdown()

	// then
	require.Equal(t, 1, len(mqClient.PublishMarshalCalls()))
}

func TestReAnnounceSeen(t *testing.T) {
	tt := []struct {
		name        string
		getSeenErr  error
		registerErr error

		expectedGetSeenCalls int
	}{
		{
			name: "success",

			expectedGetSeenCalls: 4,
		},
		{
			name:       "failed to get seen on network transactions",
			getSeenErr: errors.New("failed to get seen txs"),

			expectedGetSeenCalls: 1,
		},
		{
			name:        "failed to register transactions",
			registerErr: errors.New("failed to register txs"),

			expectedGetSeenCalls: 4,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			iterations := 0
			stop := make(chan struct{}, 1)

			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetPendingFunc: func(_ context.Context, _ time.Duration, _ time.Duration, _ time.Duration, limit int64, _ int64) ([]*store.RawTx, error) {
					require.Equal(t, int64(500), limit)

					if tc.getSeenErr != nil {
						stop <- struct{}{}

						return nil, tc.getSeenErr
					}

					if iterations >= 3 {
						stop <- struct{}{}
						return []*store.RawTx{}, nil
					}

					iterations++
					return []*store.RawTx{
						{Hash: testdata.TX1Hash},
						{Hash: testdata.TX1Hash},
						{Hash: testdata.TX1Hash},
					}, nil
				},
				SetRequestedFunc:      func(_ context.Context, _ []*chainhash.Hash) error { return nil },
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}
			pm := &mocks.MediatorMock{
				AskForTxAsyncFunc:   func(_ context.Context, _ *chainhash.Hash) {},
				AnnounceTxAsyncFunc: func(_ context.Context, _ *chainhash.Hash, _ []byte) {},
			}

			blockTxClient := &btxMocks.ClientMock{
				RegisterTransactionsFunc: func(_ context.Context, _ [][]byte) error { return tc.registerErr },
			}
			mqClient := &mqMocks.MessageQueueClientMock{
				PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error {
					return nil
				},
			}

			cStore := &cacheMocks.StoreMock{}
			sut, err := metamorph.NewProcessor(
				metamorphStore,
				cStore,
				pm,
				nil,
				metamorph.WithBlocktxClient(blockTxClient),
				metamorph.WithRegisterBatchSizeDefault(2),
				metamorph.WithMessageQueueClient(mqClient),
			)
			require.NoError(t, err)

			// when
			metamorph.ReAnnounceSeen(context.TODO(), sut)

			// then
			assert.Equal(t, tc.expectedGetSeenCalls, len(metamorphStore.GetPendingCalls()))
		})
	}
}

func TestRegisterSeen(t *testing.T) {
	tt := []struct {
		name        string
		getSeenErr  error
		registerErr error

		expectedGetSeenCalls         int
		expectedRegisterCalls        int
		expectedPublishMarshallCalls int
	}{
		{
			name: "success",

			expectedGetSeenCalls:         4,
			expectedRegisterCalls:        6,
			expectedPublishMarshallCalls: 0,
		},
		{
			name:       "failed to get seen on network transactions",
			getSeenErr: errors.New("failed to get seen txs"),

			expectedGetSeenCalls:         1,
			expectedRegisterCalls:        0,
			expectedPublishMarshallCalls: 0,
		},
		{
			name:        "failed to register transactions",
			registerErr: errors.New("failed to register txs"),

			expectedGetSeenCalls:         4,
			expectedRegisterCalls:        6,
			expectedPublishMarshallCalls: 6,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			iterations := 0
			stop := make(chan struct{}, 1)

			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetSeenFunc: func(_ context.Context, _ time.Duration, _ time.Duration, limit int64, _ int64) ([]*store.Data, error) {
					require.Equal(t, int64(50), limit)

					if tc.getSeenErr != nil {
						stop <- struct{}{}

						return nil, tc.getSeenErr
					}

					if iterations >= 3 {
						stop <- struct{}{}
						return []*store.Data{}, nil
					}

					iterations++
					return []*store.Data{
						{Hash: testdata.TX1Hash},
						{Hash: testdata.TX1Hash},
						{Hash: testdata.TX1Hash},
					}, nil
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}
			pm := &mocks.MediatorMock{}

			blockTxClient := &btxMocks.ClientMock{
				RegisterTransactionsFunc: func(_ context.Context, _ [][]byte) error { return tc.registerErr },
			}
			mqClient := &mqMocks.MessageQueueClientMock{
				PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error {
					return nil
				},
			}

			cStore := &cacheMocks.StoreMock{}
			sut, err := metamorph.NewProcessor(
				metamorphStore,
				cStore,
				pm,
				nil,
				metamorph.WithBlocktxClient(blockTxClient),
				metamorph.WithRegisterBatchSizeDefault(2),
				metamorph.WithMessageQueueClient(mqClient),
			)
			require.NoError(t, err)

			// when
			metamorph.RegisterSeenTxs(context.TODO(), sut)

			// then
			assert.Equal(t, tc.expectedGetSeenCalls, len(metamorphStore.GetSeenCalls()))
			assert.Equal(t, tc.expectedRegisterCalls, len(blockTxClient.RegisterTransactionsCalls()))
			assert.Equal(t, tc.expectedPublishMarshallCalls, len(mqClient.PublishMarshalCalls()))
		})
	}
}

func TestRejectUnconfirmedRequested(t *testing.T) {
	tt := []struct {
		name                        string
		getAndDeleteUnconfirmedErr  error
		expectedRejections          int
		expectedGetUnconfirmedCalls int
		blocks                      *blocktx_api.LatestBlocksResponse
		requestedTimes              []*chainhash.Hash
	}{
		{
			name: "success - only last tx is behind 2 blocks to be rejected",

			expectedGetUnconfirmedCalls: 1,
			expectedRejections:          3,
			blocks: &blocktx_api.LatestBlocksResponse{
				Blocks: []*blocktx_api.Block{
					{
						Height:      1000,
						Hash:        testdata.Block1Hash.CloneBytes(),
						ProcessedAt: timestamppb.New(time.Now().Add(-time.Minute * 10)),
					},
					{
						Height:      999,
						Hash:        testdata.Block2Hash.CloneBytes(),
						ProcessedAt: timestamppb.New(time.Now().Add(-time.Minute * 20)),
					},
				},
			},
			requestedTimes: []*chainhash.Hash{

				testdata.TX1Hash,

				testdata.TX2Hash,

				testdata.TX3Hash,
			},
		},
		{
			name:                        "not expected number of blocks available",
			expectedRejections:          0,
			expectedGetUnconfirmedCalls: 0,
			blocks: &blocktx_api.LatestBlocksResponse{
				Blocks: []*blocktx_api.Block{
					{
						Height:      1000,
						Hash:        testdata.Block1Hash.CloneBytes(),
						ProcessedAt: timestamppb.New(time.Now().Add(-time.Minute * 10)),
					},
				},
			},
			requestedTimes: []*chainhash.Hash{
				testdata.TX1Hash,
				testdata.TX2Hash,
				testdata.TX3Hash,
			},
		},
		{
			name:                        "skip rejecting for no old txs",
			expectedRejections:          0,
			expectedGetUnconfirmedCalls: 1,
			blocks: &blocktx_api.LatestBlocksResponse{
				Blocks: []*blocktx_api.Block{
					{
						Height:      1000,
						Hash:        testdata.Block1Hash.CloneBytes(),
						ProcessedAt: timestamppb.New(time.Now().Add(-time.Minute * 10)),
					},
					{
						Height:      999,
						Hash:        testdata.Block2Hash.CloneBytes(),
						ProcessedAt: timestamppb.New(time.Now().Add(-time.Minute * 20)),
					},
				},
			},
			requestedTimes: []*chainhash.Hash{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			i := 0
			blocktxClient := &btxMocks.ClientMock{
				LatestBlocksFunc: func(_ context.Context, _ uint64) (*blocktx_api.LatestBlocksResponse, error) {
					if i == 1 {
						return nil, errors.New("some error")
					}
					i++
					return tc.blocks, nil
				},
			}

			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetUnconfirmedRequestedFunc: func(_ context.Context, _ time.Duration, _ int64, _ int64) ([]*chainhash.Hash, error) {
					return tc.requestedTimes, nil
				},
			}
			pm := &mocks.MediatorMock{}

			statusMessageChannel := make(chan *metamorph_p2p.TxStatusMessage, 10)

			cStore := &cacheMocks.StoreMock{}
			sut, err := metamorph.NewProcessor(
				metamorphStore,
				cStore,
				pm,
				statusMessageChannel,
				metamorph.WithRejectPendingSeenEnabled(true),
				metamorph.WithBlocktxClient(blocktxClient),
				metamorph.WithRejectPendingBlocksSince(2),
			)
			require.NoError(t, err)

			// when
			metamorph.RejectUnconfirmedRequested(context.TODO(), sut)

			// then
			assert.Equal(t, tc.expectedGetUnconfirmedCalls, len(metamorphStore.GetUnconfirmedRequestedCalls()))
		})
	}
}

func TestProcessDoubleSpendTxs(t *testing.T) {
	tt := []struct {
		name           string
		doubleSpendTxs []*store.Data
		minedTxs       []*blocktx_api.IsMined
		rejected       int
	}{
		{
			name: "reject 1 double spend tx",
			doubleSpendTxs: []*store.Data{
				{
					Hash: testdata.TX1Hash,
					CompetingTxs: []string{
						testdata.TX2Hash.String(),
					},
				},
			},
			minedTxs: []*blocktx_api.IsMined{
				{
					Hash:  testdata.TX2Hash.CloneBytes(),
					Mined: true,
				},
			},
			rejected: 1,
		},
		{
			name: "reject 0 double spend tx",
			doubleSpendTxs: []*store.Data{
				{
					Hash: testdata.TX1Hash,
					CompetingTxs: []string{
						testdata.TX3Hash.String(),
					},
				},
			},
			minedTxs: []*blocktx_api.IsMined{
				{
					Hash:  testdata.TX2Hash.CloneBytes(),
					Mined: false,
				},
			},
			rejected: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetDoubleSpendTxsFunc: func(_ context.Context, _ time.Time) ([]*store.Data, error) {
					return tc.doubleSpendTxs, nil
				},
				UpdateStatusFunc: func(_ context.Context, _ []store.UpdateStatus) ([]*store.Data, error) {
					return nil, nil
				},
			}
			pm := &mocks.MediatorMock{}
			blockTxClient := &btxMocks.ClientMock{
				AnyTransactionsMinedFunc: func(_ context.Context, _ [][]byte) ([]*blocktx_api.IsMined, error) {
					return tc.minedTxs, nil
				},
			}

			statusMessageChannel := make(chan *metamorph_p2p.TxStatusMessage, 10)

			cStore := &cacheMocks.StoreMock{}
			sut, err := metamorph.NewProcessor(
				metamorphStore,
				cStore,
				pm,
				statusMessageChannel,
				metamorph.WithRejectPendingSeenEnabled(true),
				metamorph.WithBlocktxClient(blockTxClient),
			)
			require.NoError(t, err)

			// when
			metamorph.ProcessDoubleSpendTxs(context.TODO(), sut)

			// then
			assert.Equal(t, tc.rejected, len(metamorphStore.UpdateStatusCalls()))
		})
	}
}

func TestProcessorHealth(t *testing.T) {
	tt := []struct {
		name       string
		peersAdded uint

		expectedError error
	}{
		{
			name:       "3 healthy peers",
			peersAdded: 3,
		},
		{
			name:       "1 healthy peer",
			peersAdded: 1,

			expectedError: metamorph.ErrUnhealthy,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetFunc: func(_ context.Context, _ []byte) (*store.Data, error) {
					return &store.Data{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
				GetUnseenFunc: func(_ context.Context, _ time.Time, _ int64, offset int64) ([]*store.Data, error) {
					if offset != 0 {
						return nil, nil
					}
					return []*store.Data{
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

			messenger := &mocks.MediatorMock{
				CountConnectedPeersFunc: func() uint { return tc.peersAdded },
			}
			sut, err := metamorph.NewProcessor(metamorphStore, nil, messenger, nil,
				metamorph.WithReAnnounceUnseenInterval(time.Millisecond*20),
				metamorph.WithNow(func() time.Time {
					return time.Date(2033, 1, 1, 1, 0, 0, 0, time.UTC)
				}),
			)
			require.NoError(t, err)
			defer sut.Shutdown()

			// when
			actualError := sut.Health()

			// then
			require.ErrorIs(t, actualError, tc.expectedError)
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
			topic:    mq.MinedTxsTopic,
			topicErr: map[string]error{mq.MinedTxsTopic: errors.New("failed to subscribe")},

			expectedError: metamorph.ErrFailedToSubscribe,
		},
		{
			name:     "error - subscribe submit txs",
			topic:    mq.SubmitTxTopic,
			topicErr: map[string]error{mq.SubmitTxTopic: errors.New("failed to subscribe")},

			expectedError: metamorph.ErrFailedToSubscribe,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) {
				return 0, nil
			}}

			pm := &mocks.MediatorMock{}

			cStore := &cacheMocks.StoreMock{}

			var subscribeMinedTxsFunction func([]byte) error
			var subscribeSubmitTxsFunction func([]byte) error
			mqClient := &mqMocks.MessageQueueClientMock{
				ConsumeFunc: func(topic string, msgFunc func([]byte) error) error {
					subscribeSubmitTxsFunction = msgFunc

					err, ok := tc.topicErr[topic]
					if ok {
						return err
					}
					return nil
				},
				QueueSubscribeFunc: func(topic string, msgFunc func([]byte) error) error {
					subscribeMinedTxsFunction = msgFunc

					err, ok := tc.topicErr[topic]
					if ok {
						return err
					}
					return nil
				},
			}

			submittedTxsChan := make(chan *metamorph_api.PostTransactionRequest, 2)
			minedTxsChan := make(chan *blocktx_api.TransactionBlocks, 2)

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, pm, nil,
				metamorph.WithMessageQueueClient(mqClient),
				metamorph.WithSubmittedTxsChan(submittedTxsChan),
				metamorph.WithMinedTxsChan(minedTxsChan),
			)
			require.NoError(t, err)

			// when
			actualError := sut.Start(false)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				require.ErrorContains(t, actualError, tc.topic)
				return
			}
			require.NoError(t, actualError)

			txBlock := &blocktx_api.TransactionBlock{}
			data, err := proto.Marshal(txBlock)
			require.NoError(t, err)

			_ = subscribeMinedTxsFunction([]byte("invalid data"))
			_ = subscribeMinedTxsFunction(data)

			txRequest := &metamorph_api.PostTransactionRequest{}
			data, err = proto.Marshal(txRequest)
			require.NoError(t, err)
			_ = subscribeSubmitTxsFunction([]byte("invalid data"))
			_ = subscribeSubmitTxsFunction(data)

			sut.Shutdown()
		})
	}
}
