package metamorph_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	metamorph_p2p "github.com/bitcoin-sv/arc/internal/metamorph/bcnet/p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
	"github.com/bitcoin-sv/arc/internal/p2p"
	p2pMocks "github.com/bitcoin-sv/arc/internal/p2p/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
)

func TestNewProcessor(t *testing.T) {
	mtmStore := &storeMocks.MetamorphStoreMock{
		GetFunc: func(_ context.Context, _ []byte) (*store.Data, error) {
			return &store.Data{Hash: testdata.TX2Hash}, nil
		},
		SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
	}

	nMessenger := &p2p.NetworkMessenger{}
	cStore := cache.NewMemoryStore()

	tt := []struct {
		name      string
		store     store.MetamorphStore
		messenger *p2p.NetworkMessenger

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

			expectedError: metamorph.ErrPeerMessangerNil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// when
			sut, actualErr := metamorph.NewProcessor(tc.store, cStore, tc.messenger, nil,
				metamorph.WithCacheExpiryTime(time.Second*5),
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

			expectedSetLockedCallsGreaterThan: 2,
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
					require.Equal(t, int64(5000), limit)
					return tc.setLockedErr
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}

			messenger := &p2p.NetworkMessenger{}
			cStore := cache.NewMemoryStore()

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

		expectedResponses     []metamorph_api.Status
		expectedSetCalls      int
		expectedAnnounceCalls int
		expectedRequestCalls  int
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
				GetUnminedFunc: func(_ context.Context, _ time.Time, _ int64, offset int64) ([]*store.Data, error) {
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

			announceMsgCounter := &atomic.Int32{}
			requestMsgCounter := &atomic.Int32{}

			peer := &p2pMocks.PeerIMock{
				WriteMsgFunc: func(msg wire.Message) {
					if msg.Command() == wire.CmdInv {
						announceMsgCounter.Add(1)
					} else if msg.Command() == wire.CmdGetData {
						requestMsgCounter.Add(1)
					}
				},
				NetworkFunc:   func() wire.BitcoinNet { return wire.TestNet },
				StringFunc:    func() string { return "peer" },
				ConnectedFunc: func() bool { return true },
			}

			cStore := cache.NewMemoryStore()

			pm := p2p.NewPeerManager(slog.Default(), wire.TestNet)
			err := pm.AddPeer(peer)
			require.NoError(t, err)

			messenger := p2p.NewNetworkMessenger(slog.Default(), pm)

			publisher := &mocks.MessageQueueClientMock{
				PublishFunc: func(_ context.Context, _ string, _ []byte) error {
					return nil
				},
			}

			sut, err := metamorph.NewProcessor(s, cStore, messenger, nil, metamorph.WithMessageQueueClient(publisher))
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
					Data: &store.Data{
						Hash: testdata.TX1Hash,
					},
					ResponseChannel: responseChannel,
				})
			wg.Wait()
			time.Sleep(250 * time.Millisecond)
			// then
			require.Equal(t, tc.expectedSetCalls, len(s.SetCalls()))
			require.Equal(t, tc.expectedAnnounceCalls, int(announceMsgCounter.Load()))
			require.Equal(t, tc.expectedRequestCalls, int(requestMsgCounter.Load()))
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
		updateResp [][]*store.Data

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
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			counter := 0
			callbackSent := make(chan struct{}, tc.expectedCallbacks)

			metamorphStore := &storeMocks.MetamorphStoreMock{
				GetFunc: func(_ context.Context, _ []byte) (*store.Data, error) {
					return &store.Data{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
				UpdateStatusBulkFunc: func(_ context.Context, _ []store.UpdateStatus) ([]*store.Data, error) {
					if len(tc.updateResp) > 0 {
						counter++
						return tc.updateResp[counter-1], tc.updateErr
					}
					return nil, tc.updateErr
				},
				UpdateDoubleSpendFunc: func(_ context.Context, _ []store.UpdateStatus) ([]*store.Data, error) {
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

			messenger := &p2p.NetworkMessenger{}
			cStore := cache.NewMemoryStore()

			callbackSender := &mocks.CallbackSenderMock{
				SendCallbackFunc: func(_ context.Context, _ *store.Data) {
					callbackSent <- struct{}{}
				},
			}

			statusMessageChannel := make(chan *metamorph_p2p.PeerTxMessage, 10)

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, messenger, statusMessageChannel,
				metamorph.WithNow(func() time.Time { return time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC) }),
				metamorph.WithProcessStatusUpdatesInterval(200*time.Millisecond),
				metamorph.WithProcessStatusUpdatesBatchSize(3),
				metamorph.WithCallbackSender(callbackSender),
			)
			require.NoError(t, err)

			// when
			sut.StartProcessStatusUpdatesInStorage()
			sut.StartSendStatusUpdate()

			assert.Equal(t, 0, sut.GetProcessorMapSize())
			for _, testInput := range tc.inputs {
				statusMessageChannel <- &metamorph_p2p.PeerTxMessage{
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

		expectedSetBulkCalls int
		expectedInvMessages  int
		expcetedInvInMessage int
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

			expectedSetBulkCalls: 1,
			expectedInvMessages:  1,
			expcetedInvInMessage: 2,
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

			expectedSetBulkCalls: 2,
			expectedInvMessages:  1,
			expcetedInvInMessage: 5,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			wg := &sync.WaitGroup{}

			updateBulkCounter := 0
			s := &storeMocks.MetamorphStoreMock{
				SetBulkFunc: func(_ context.Context, _ []*store.Data) error {
					return nil
				},
				UpdateStatusBulkFunc: func(_ context.Context, updates []store.UpdateStatus) ([]*store.Data, error) {
					for _, u := range updates {
						require.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, u.Status)
					}
					updateBulkCounter++

					if updateBulkCounter >= tc.expectedSetBulkCalls {
						wg.Done()
					}
					return nil, nil
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}

			announceMsgCounter := &atomic.Int32{}
			peer := &p2pMocks.PeerIMock{
				WriteMsgFunc: func(msg wire.Message) {
					if msg.Command() == wire.CmdInv {
						announceMsgCounter.Add(1)

						invMsg, ok := msg.(*wire.MsgInv)
						require.True(t, ok)
						require.Equal(t, tc.expcetedInvInMessage, len(invMsg.InvList))
					}
				},
				NetworkFunc:   func() wire.BitcoinNet { return wire.TestNet },
				StringFunc:    func() string { return "peer" },
				ConnectedFunc: func() bool { return true },
			}

			cStore := cache.NewMemoryStore()

			pm := p2p.NewPeerManager(slog.Default(), wire.TestNet)
			err := pm.AddPeer(peer)
			require.NoError(t, err)

			messenger := p2p.NewNetworkMessenger(slog.Default(), pm)

			publisher := &mocks.MessageQueueClientMock{
				PublishFunc: func(_ context.Context, _ string, _ []byte) error {
					return nil
				},
			}
			const submittedTxsBuffer = 5
			submittedTxsChan := make(chan *metamorph_api.TransactionRequest, submittedTxsBuffer)
			sut, err := metamorph.NewProcessor(s, cStore, messenger, nil,
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
			time.Sleep(250 * time.Millisecond)
			// then
			require.Equal(t, tc.expectedSetBulkCalls, len(s.SetBulkCalls()))
			require.Equal(t, tc.expectedInvMessages, int(announceMsgCounter.Load()))
		})
	}
}

func TestProcessExpiredTransactions(t *testing.T) {
	tt := []struct {
		name          string
		retries       int
		getUnminedErr error

		expectedRequests      int32
		expectedAnnouncements int32
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
				GetUnminedFunc: func(_ context.Context, _ time.Time, _ int64, offset int64) ([]*store.Data, error) {
					if offset != 0 {
						return nil, nil
					}
					unminedData := []*store.Data{
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

					return unminedData, tc.getUnminedErr
				},
				IncrementRetriesBulkFunc: func(_ context.Context, _ []*chainhash.Hash) error {
					retries++
					return nil
				},
			}

			var announceMsgCounter atomic.Int32
			var requestMsgCounter atomic.Int32
			peer := &p2pMocks.PeerIMock{
				WriteMsgFunc: func(msg wire.Message) {
					if msg.Command() == wire.CmdInv {
						announceMsgCounter.Add(1)
					} else if msg.Command() == wire.CmdGetData {
						requestMsgCounter.Add(1)
					}
				},
				NetworkFunc:   func() wire.BitcoinNet { return wire.TestNet },
				StringFunc:    func() string { return "peer" },
				ConnectedFunc: func() bool { return true },
			}

			cStore := cache.NewMemoryStore()

			pm := p2p.NewPeerManager(slog.Default(), wire.TestNet)
			err := pm.AddPeer(peer)
			require.NoError(t, err)

			messenger := p2p.NewNetworkMessenger(slog.Default(), pm)

			publisher := &mocks.MessageQueueClientMock{
				PublishFunc: func(_ context.Context, _ string, _ []byte) error {
					return nil
				},
			}

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, messenger, nil,
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
			require.Equal(t, tc.expectedAnnouncements, announceMsgCounter.Load())
			require.Equal(t, tc.expectedRequests, requestMsgCounter.Load())
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
						{Callbacks: []store.Callback{{CallbackURL: "http://callback.com"}}},
						{Callbacks: []store.Callback{{CallbackURL: "http://callback.com"}}},
						{},
					}, tc.updateMinedErr
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}
			pm := &p2p.NetworkMessenger{}
			minedTxsChan := make(chan *blocktx_api.TransactionBlock, 5)
			callbackSender := &mocks.CallbackSenderMock{
				SendCallbackFunc: func(_ context.Context, _ *store.Data) {},
			}
			cStore := cache.NewMemoryStore()
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
				GetFunc: func(_ context.Context, _ []byte) (*store.Data, error) {
					return &store.Data{Hash: testdata.TX2Hash}, nil
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
				GetUnminedFunc: func(_ context.Context, _ time.Time, _ int64, offset int64) ([]*store.Data, error) {
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

			pm := p2p.NewPeerManager(slog.Default(), wire.TestNet)
			for range tc.peersAdded {
				peer := p2pMocks.PeerIMock{
					NetworkFunc:   func() wire.BitcoinNet { return wire.TestNet },
					ConnectedFunc: func() bool { return true },
					StringFunc:    func() string { return "peer" },
				}
				require.NoError(t, pm.AddPeer(&peer))
			}
			cStore := cache.NewMemoryStore()

			messanger := p2p.NewNetworkMessenger(slog.Default(), pm)

			sut, err := metamorph.NewProcessor(metamorphStore, cStore, messanger, nil,
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
			metamorphStore := &storeMocks.MetamorphStoreMock{SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) {
				return 0, nil
			}}

			pm := &p2p.NetworkMessenger{}

			cStore := cache.NewMemoryStore()

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

			txRequest := &metamorph_api.TransactionRequest{}
			data, err = proto.Marshal(txRequest)
			require.NoError(t, err)
			_ = subscribeSubmitTxsFunction([]byte("invalid data"))
			_ = subscribeSubmitTxsFunction(data)

			sut.Shutdown()
		})
	}
}
