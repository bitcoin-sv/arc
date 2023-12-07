package metamorph_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	. "github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	. "github.com/bitcoin-sv/arc/metamorph/mocks"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/metamorph/store/badger"
	metamorphSql "github.com/bitcoin-sv/arc/metamorph/store/sql"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/labstack/gommon/random"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:generate moq -pkg mocks -out ./mocks/store_mock.go ./store/ MetamorphStore
//go:generate moq -pkg mocks -out ./mocks/blocktx_mock.go ../blocktx/ ClientI

func TestNewProcessor(t *testing.T) {
	mtmStore := &MetamorphStoreMock{
		SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error { return nil },
	}

	pm := p2p.NewPeerManagerMock()

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

			processor, err := NewProcessor(tc.store, tc.pm, nil, nil,
				WithCacheExpiryTime(time.Second*5),
				WithProcessExpiredSeenTxsInterval(time.Second*5),
				WithProcessorLogger(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault}))),
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
		name                   string
		storedData             []store.StoreData
		isCentralised          bool
		updateStatusErr        error
		getTransactionBlockErr error
		delErr                 error

		expectedDeletions         int
		expectedItemTxHashesFinal []*chainhash.Hash
	}{
		{
			name: "no unmined transactions loaded",
		},
		{
			name: "load 3 unmined transactions, TX2 was mined",
			storedData: []store.StoreData{
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX1Hash,
					Status:      metamorph_api.Status_SENT_TO_NETWORK,
				},
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX2Hash,
					Status:      metamorph_api.Status_SEEN_ON_NETWORK,
					CallbackUrl: "http://api.example.com",
				},
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX3Hash,
					Status:      metamorph_api.Status_SEEN_ON_NETWORK,
				},
			},

			expectedItemTxHashesFinal: []*chainhash.Hash{testdata.TX1Hash, testdata.TX3Hash},
		},
		{
			name: "load 2 unmined transactions, none mined",
			storedData: []store.StoreData{
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

			expectedItemTxHashesFinal: []*chainhash.Hash{testdata.TX1Hash, testdata.TX2Hash},
		},
		{
			name: "update status fails",
			storedData: []store.StoreData{
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX2Hash,
					Status:      metamorph_api.Status_STORED,
				},
			},
			updateStatusErr: errors.New("failed to update status"),

			expectedItemTxHashesFinal: []*chainhash.Hash{testdata.TX2Hash},
		},
		{
			name: "get transaction block fails",
			storedData: []store.StoreData{
				{
					StoredAt:    storedAt,
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX2Hash,
					Status:      metamorph_api.Status_SEEN_ON_NETWORK,
				},
			},
			getTransactionBlockErr: errors.New("failed to get transaction block"),

			expectedItemTxHashesFinal: []*chainhash.Hash{testdata.TX2Hash},
		},
		{
			name: "delete expired",
			storedData: []store.StoreData{
				{
					StoredAt:    storedAt.Add(-400 * time.Hour),
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX2Hash,
					Status:      metamorph_api.Status_SEEN_ON_NETWORK,
				},
			},

			expectedDeletions:         1,
			expectedItemTxHashesFinal: []*chainhash.Hash{},
		},
		{
			name: "delete expired - deletion fails",
			storedData: []store.StoreData{
				{
					StoredAt:    storedAt.Add(-400 * time.Hour),
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX2Hash,
					Status:      metamorph_api.Status_SEEN_ON_NETWORK,
				},
			},
			delErr: errors.New("failed to delete hash"),

			expectedDeletions:         1,
			expectedItemTxHashesFinal: []*chainhash.Hash{testdata.TX2Hash},
		},
		{
			name:          "delete expired - centralised storage",
			isCentralised: true,
			storedData: []store.StoreData{
				{
					StoredAt:    storedAt.Add(-400 * time.Hour),
					AnnouncedAt: storedAt.Add(1 * time.Second),
					Hash:        testdata.TX2Hash,
					Status:      metamorph_api.Status_SEEN_ON_NETWORK,
				},
			},

			expectedDeletions:         0,
			expectedItemTxHashesFinal: []*chainhash.Hash{testdata.TX2Hash},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			pm := p2p.NewPeerManagerMock()

			btxMock := &ClientIMock{
				GetTransactionBlockFunc: func(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.RegisterTransactionResponse, error) {

					var txResponse *blocktx_api.RegisterTransactionResponse

					// TX2 was mined
					if bytes.Equal(testdata.TX2Hash[:], transaction.Hash[:]) {
						txResponse = &blocktx_api.RegisterTransactionResponse{
							BlockHash:   testdata.Block2Hash[:],
							BlockHeight: 2,
						}
					}

					return txResponse, tc.getTransactionBlockErr
				},
			}
			mtmStore := &MetamorphStoreMock{
				GetUnminedTransactionsFunc: func(contextMoqParam context.Context) ([]store.StoreData, error) {
					return tc.storedData, nil
				},
				UpdateMinedFunc: func(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
					require.Equal(t, testdata.TX2Hash, hash)
					return nil
				},
				UpdateStatusFunc: func(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
					require.Equal(t, bytes.Compare(testdata.TX2Hash[:], hash[:]), 0)
					return tc.updateStatusErr
				},
				GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
					return &store.StoreData{
						StoredAt:    storedAt,
						AnnouncedAt: storedAt.Add(1 * time.Second),
						Hash:        testdata.TX2Hash,
						Status:      metamorph_api.Status_SEEN_ON_NETWORK,
						CallbackUrl: "http://api.example.com",
						BlockHash:   testdata.Block2Hash,
						BlockHeight: 2,
					}, nil
				},
				DelFunc: func(ctx context.Context, key []byte) error {
					require.Equal(t, testdata.TX2Hash[:], key)
					return tc.delErr
				},
				SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error {
					require.ElementsMatch(t, tc.expectedItemTxHashesFinal, hashes)
					require.Equal(t, len(tc.expectedItemTxHashesFinal), len(hashes))
					return nil
				},
				IsCentralisedFunc: func() bool {
					return tc.isCentralised
				},
			}

			processor, err := NewProcessor(mtmStore, pm, nil, btxMock,
				WithProcessExpiredSeenTxsInterval(time.Hour*24),
				WithCacheExpiryTime(time.Hour*24),
				WithNow(func() time.Time {
					return storedAt.Add(1 * time.Hour)
				}),
				WithDataRetentionPeriod(time.Hour*24),
			)
			require.NoError(t, err)
			defer processor.Shutdown()
			require.Equal(t, 0, processor.ProcessorResponseMap.Len())
			processor.LoadUnmined()

			allItemHashes := make([]*chainhash.Hash, 0, len(processor.ProcessorResponseMap.Items()))

			for i, item := range processor.ProcessorResponseMap.Items() {
				require.Equal(t, i, *item.Hash)
				allItemHashes = append(allItemHashes, item.Hash)
			}

			require.Equal(t, tc.expectedDeletions, len(mtmStore.DelCalls()))
			require.ElementsMatch(t, tc.expectedItemTxHashesFinal, allItemHashes)
		})
	}
}

func TestProcessTransaction(t *testing.T) {
	t.Run("ProcessTransaction", func(t *testing.T) {
		s, err := metamorphSql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

		expectedResponses := []metamorph_api.Status{
			metamorph_api.Status_RECEIVED,
			metamorph_api.Status_STORED,
			metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		}

		responseChannel := make(chan processor_response.StatusAndError)

		var wg sync.WaitGroup
		wg.Add(len(expectedResponses))
		go func() {
			n := 0
			for response := range responseChannel {
				status := response.Status
				assert.Equal(t, testdata.TX1Hash, response.Hash)
				assert.Equalf(t, expectedResponses[n], status, "Iteration %d: Expected %s, got %s", n, expectedResponses[n].String(), status.String())
				wg.Done()
				n++
			}
		}()

		processor.ProcessTransaction(context.TODO(), &ProcessorRequest{
			Data: &store.StoreData{
				Hash: testdata.TX1Hash,
			},
			ResponseChannel: responseChannel,
		})
		wg.Wait()

		assert.Equal(t, 1, processor.ProcessorResponseMap.Len())
		items := processor.ProcessorResponseMap.Items()
		assert.Equal(t, testdata.TX1Hash, items[*testdata.TX1Hash].Hash)
		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, items[*testdata.TX1Hash].Status)

		assert.Len(t, pm.AnnouncedTransactions, 1)
		assert.Equal(t, testdata.TX1Hash, pm.AnnouncedTransactions[0])

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, testdata.TX1Hash, txStored.Hash)
	})
}

func Benchmark_ProcessTransaction(b *testing.B) {
	s, err := metamorphSql.New("sqlite_memory") // prevents profiling database code
	require.NoError(b, err)

	pm := p2p.NewPeerManagerMock()

	processor, err := NewProcessor(s, pm, nil, nil)
	require.NoError(b, err)
	assert.Equal(b, 0, processor.ProcessorResponseMap.Len())

	btTx, _ := bt.NewTxFromBytes(testdata.TX1RawBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		btTx.Inputs[0].SequenceNumber = uint32(i)
		hash, _ := chainhash.NewHashFromStr(btTx.TxID())
		processor.ProcessTransaction(context.TODO(), &ProcessorRequest{
			Data: &store.StoreData{
				Hash: hash,
			},
		})
	}
}

func TestSendStatusForTransaction(t *testing.T) {
	t.Run("SendStatusForTransaction unknown tx", func(t *testing.T) {
		s, err := metamorphSql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(testdata.TX1Hash, metamorph_api.Status_MINED, "test", nil)
		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())
	})

	t.Run("SendStatusForTransaction err", func(t *testing.T) {
		s, err := metamorphSql.New("sqlite_memory")
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

		throwErr := fmt.Errorf("some error")
		ok, sendErr := processor.SendStatusForTransaction(testdata.TX1Hash, metamorph_api.Status_REJECTED, "test", throwErr)
		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())
	})

	t.Run("SendStatusForTransaction known tx - no update", func(t *testing.T) {
		s, err := metamorphSql.New("sqlite_memory")
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(testdata.TX1Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "test", nil)
		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, txStored.Status)
	})

	t.Run("SendStatusForTransaction known tx - processed", func(t *testing.T) {
		s, err := metamorphSql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

		responseChannel := make(chan processor_response.StatusAndError)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for response := range responseChannel {
				status := response.Status
				fmt.Printf("response: %s\n", status)
				if status == metamorph_api.Status_ANNOUNCED_TO_NETWORK {
					close(responseChannel)
					wg.Done()
					return
				}
			}
		}()

		processor.ProcessTransaction(context.TODO(), &ProcessorRequest{
			Data: &store.StoreData{
				Hash: testdata.TX1Hash,
			},
			ResponseChannel: responseChannel,
		})
		wg.Wait()

		assert.Equal(t, 1, processor.ProcessorResponseMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(testdata.TX1Hash, metamorph_api.Status_MINED, "test", nil)
		// need to sleep, since everything is async
		time.Sleep(100 * time.Millisecond)

		assert.True(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len(), "should have been removed from the map")

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_MINED, txStored.Status)
	})
}

func TestSendStatusMinedForTransaction(t *testing.T) {
	t.Run("SendStatusMinedForTransaction known tx", func(t *testing.T) {
		s, err := metamorphSql.New("sqlite_memory")
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, nil, nil)
		require.NoError(t, err)
		processor.ProcessorResponseMap.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(
			testdata.TX1Hash,
			metamorph_api.Status_SEEN_ON_NETWORK,
		))
		assert.Equal(t, 1, processor.ProcessorResponseMap.Len())

		ok, sendErr := processor.SendStatusMinedForTransaction(testdata.TX1Hash, testdata.Block1Hash, 1233)
		time.Sleep(100 * time.Millisecond)
		assert.True(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_MINED, txStored.Status)
	})

	t.Run("SendStatusMinedForTransaction callback", func(t *testing.T) {
		s, err := metamorphSql.New("sqlite_memory")
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock()

		var wg sync.WaitGroup
		callbackCh := make(chan *callbacker_api.Callback)
		wg.Add(1)
		go func() {
			for cb := range callbackCh {
				assert.Equal(t, metamorph_api.Status_MINED, metamorph_api.Status(cb.Status))
				assert.Equal(t, testdata.TX1Hash.CloneBytes(), cb.Hash)
				assert.Equal(t, testdata.Block1Hash[:], cb.BlockHash)
				assert.Equal(t, uint64(1233), cb.BlockHeight)
				assert.Equal(t, "https://test.com", cb.Url)
				assert.Equal(t, "token", cb.Token)
				wg.Done()
			}
		}()

		processor, err := NewProcessor(s, pm, callbackCh, nil)
		require.NoError(t, err)
		// add the tx to the map
		processor.ProcessorResponseMap.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(
			testdata.TX1Hash,
			metamorph_api.Status_SEEN_ON_NETWORK,
		))

		ok, sendErr := processor.SendStatusMinedForTransaction(testdata.TX1Hash, testdata.Block1Hash, 1233)
		time.Sleep(100 * time.Millisecond)
		assert.True(t, ok)
		assert.NoError(t, sendErr)

		wg.Wait()
	})

	t.Run("SendStatusForTransaction known tx - processed", func(t *testing.T) {
		s, err := metamorphSql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len())

		responseChannel := make(chan processor_response.StatusAndError)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for response := range responseChannel {
				status := response.Status
				fmt.Printf("response: %s\n", status)
				if status == metamorph_api.Status_ANNOUNCED_TO_NETWORK {
					close(responseChannel)
					time.Sleep(1 * time.Millisecond)
					wg.Done()
					return
				}
			}
		}()

		processor.ProcessTransaction(context.TODO(), &ProcessorRequest{
			Data: &store.StoreData{
				Hash: testdata.TX1Hash,
			},
			ResponseChannel: responseChannel,
		})
		wg.Wait()

		assert.Equal(t, 1, processor.ProcessorResponseMap.Len())

		ok, sendErr := processor.SendStatusMinedForTransaction(testdata.TX1Hash, testdata.Block1Hash, 1233)
		time.Sleep(10 * time.Millisecond)
		assert.True(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.ProcessorResponseMap.Len(), "should have been removed from the map")

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_MINED, txStored.Status)
	})
}

func BenchmarkProcessTransaction(b *testing.B) {
	direName := fmt.Sprintf("./test-benchmark-%s", random.String(6))
	s, err := badger.New(direName)
	require.NoError(b, err)
	defer func() {
		_ = s.Close(context.Background())
		_ = os.RemoveAll(direName)
	}()

	pm := p2p.NewPeerManagerMock()
	processor, err := NewProcessor(s, pm, nil, nil)
	require.NoError(b, err)
	assert.Equal(b, 0, processor.ProcessorResponseMap.Len())

	txs := make(map[string]*chainhash.Hash)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txID := fmt.Sprintf("%x", i)

		txHash := chainhash.HashH([]byte(txID))

		txs[txID] = &txHash

		processor.ProcessTransaction(context.TODO(), &ProcessorRequest{
			Data: &store.StoreData{
				Hash:   &txHash,
				Status: metamorph_api.Status_UNKNOWN,
				RawTx:  testdata.TX1RawBytes,
			},
		})
	}
	b.StopTimer()

	// wait for the last ResponseItems to be written to the store
	time.Sleep(1 * time.Second)
}

func TestProcessExpiredSeenTransactions(t *testing.T) {
	txsBlocks := []*blocktx_api.TransactionBlock{
		{
			BlockHash:       testdata.Block1Hash[:],
			BlockHeight:     1234,
			TransactionHash: testdata.TX1Hash[:],
		},
		{
			BlockHash:       testdata.Block1Hash[:],
			BlockHeight:     1234,
			TransactionHash: testdata.TX2Hash[:],
		},
		{
			BlockHash:       testdata.Block1Hash[:],
			BlockHeight:     1234,
			TransactionHash: testdata.TX3Hash[:],
		},
	}

	tt := []struct {
		name                    string
		blocks                  []*blocktx_api.TransactionBlock
		getTransactionBlocksErr error
		updateMinedErr          error

		expectedNrOfUpdates         int
		expectedNrOfBlockTxRequests int
	}{
		{
			name:   "expired seen txs",
			blocks: txsBlocks,

			expectedNrOfUpdates:         3,
			expectedNrOfBlockTxRequests: 1,
		},
		{
			name:                    "failed to get transaction blocks",
			getTransactionBlocksErr: errors.New("failed to get transaction blocks"),

			expectedNrOfUpdates:         0,
			expectedNrOfBlockTxRequests: 1,
		},
		{
			name: "failed to parse block hash",
			blocks: []*blocktx_api.TransactionBlock{{
				BlockHash: []byte("not a valid block hash"),
			}},

			expectedNrOfUpdates:         0,
			expectedNrOfBlockTxRequests: 1,
		},
		{
			name:           "failed to update mined",
			blocks:         txsBlocks,
			updateMinedErr: errors.New("failed to update mined"),

			expectedNrOfUpdates:         3,
			expectedNrOfBlockTxRequests: 1,
		},
		{
			name: "failed to get tx from response map",
			blocks: []*blocktx_api.TransactionBlock{
				{
					BlockHash:       testdata.Block1Hash[:],
					BlockHeight:     1234,
					TransactionHash: testdata.TX4Hash[:],
				},
			},

			expectedNrOfUpdates:         0,
			expectedNrOfBlockTxRequests: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			metamorphStore := &MetamorphStoreMock{
				UpdateMinedFunc: func(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
					require.Condition(t, func() (success bool) {
						oneOfHash := hash.IsEqual(testdata.TX1Hash) || hash.IsEqual(testdata.TX2Hash) || hash.IsEqual(testdata.TX3Hash)
						isBlockHeight := blockHeight == 1234
						isBlockHash := blockHash.IsEqual(testdata.Block1Hash)
						return oneOfHash && isBlockHeight && isBlockHash
					})

					return tc.updateMinedErr
				},
				SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error { return nil },
			}
			btxMock := &ClientIMock{
				GetTransactionBlocksFunc: func(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
					require.Equal(t, 3, len(transaction.Transactions))

					return &blocktx_api.TransactionBlocks{TransactionBlocks: tc.blocks}, tc.getTransactionBlocksErr
				},
			}

			pm := p2p.NewPeerManagerMock()
			processor, err := NewProcessor(metamorphStore, pm, nil, btxMock,
				WithProcessExpiredSeenTxsInterval(20*time.Millisecond),
				WithProcessExpiredTxsInterval(time.Hour),
			)
			require.NoError(t, err)
			defer processor.Shutdown()

			require.Equal(t, 0, processor.ProcessorResponseMap.Len())

			processor.ProcessorResponseMap.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SEEN_ON_NETWORK))
			processor.ProcessorResponseMap.Set(testdata.TX2Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_SEEN_ON_NETWORK))
			processor.ProcessorResponseMap.Set(testdata.TX3Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX3Hash, metamorph_api.Status_SEEN_ON_NETWORK))

			time.Sleep(25 * time.Millisecond)

			require.Equal(t, tc.expectedNrOfUpdates, len(metamorphStore.UpdateMinedCalls()))
			require.Equal(t, tc.expectedNrOfBlockTxRequests, len(btxMock.GetTransactionBlocksCalls()))
		})
	}
}

func TestProcessExpiredTransactions(t *testing.T) {
	tt := []struct {
		name    string
		retries uint32
	}{
		{
			name:    "expired txs - 0 retries",
			retries: 0,
		},
		{
			name:    "expired txs - 0 retries",
			retries: 16,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphStore := &MetamorphStoreMock{SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error { return nil }}
			pm := p2p.NewPeerManagerMock()
			processor, err := NewProcessor(metamorphStore, pm, nil, nil,
				WithProcessExpiredSeenTxsInterval(time.Hour),
				WithProcessExpiredTxsInterval(time.Millisecond*20),
				WithNow(func() time.Time {
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
		})
	}
}
