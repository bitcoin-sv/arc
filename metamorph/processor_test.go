package metamorph

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	blockTxMock "github.com/bitcoin-sv/arc/metamorph/blocktx/mock"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/metamorph/store/badger"
	storeMock "github.com/bitcoin-sv/arc/metamorph/store/mock"
	"github.com/bitcoin-sv/arc/metamorph/store/sql"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/labstack/gommon/random"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProcessor(t *testing.T) {
	t.Run("NewProcessor", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)

		if processor == nil {
			t.Error("Expected a non-nil Processor")
		}
	})

	t.Run("NewProcessor - err no store", func(t *testing.T) {
		_, err := NewProcessor(nil, nil, "test", nil, nil)
		require.ErrorContains(t, err, "store cannot be nil")
	})

	t.Run("NewProcessor - err no peer manager", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		_, err = NewProcessor(s, nil, "test", nil, nil)
		require.ErrorContains(t, err, "peer manager cannot be nil")
	})
}

func TestLoadUnseen(t *testing.T) {
	t.Run("LoadUnmined empty", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.processorResponseMap.Len())
		processor.LoadUnmined()
		assert.Equal(t, 0, processor.processorResponseMap.Len())
	})

	t.Run("LoadUnmined", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock()

		blockTxMock := NewBlockTxMock("test-address")
		processor, err := NewProcessor(s, pm, "test", nil, blockTxMock)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.processorResponseMap.Len())
		processor.LoadUnmined()
		assert.Equal(t, 2, processor.processorResponseMap.Len())
		items := processor.processorResponseMap.Items()
		assert.Equal(t, testdata.TX1Hash, items[*testdata.TX1Hash].Hash)
		assert.Equal(t, testdata.TX2Hash, items[*testdata.TX2Hash].Hash)
	})
}

func TestProcessTransaction(t *testing.T) {
	t.Run("ProcessTransaction", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

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

		processor.ProcessTransaction(NewProcessorRequest(
			context.Background(),
			&store.StoreData{
				Hash: testdata.TX1Hash,
			},
			responseChannel,
		))
		wg.Wait()

		assert.Equal(t, 1, processor.processorResponseMap.Len())
		items := processor.processorResponseMap.Items()
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
	s, err := sql.New("sqlite_memory") // prevents profiling database code
	require.NoError(b, err)

	pm := p2p.NewPeerManagerMock()

	processor, err := NewProcessor(s, pm, "test", nil, nil)
	require.NoError(b, err)
	assert.Equal(b, 0, processor.processorResponseMap.Len())

	btTx, _ := bt.NewTxFromBytes(testdata.TX1RawBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		btTx.Inputs[0].SequenceNumber = uint32(i)
		hash, _ := chainhash.NewHashFromStr(btTx.TxID())
		processor.ProcessTransaction(NewProcessorRequest(
			context.Background(),
			&store.StoreData{
				Hash: hash,
			},
			nil,
		))
	}
}

func TestSendStatusForTransaction(t *testing.T) {
	t.Run("SendStatusForTransaction unknown tx", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(testdata.TX1Hash, metamorph_api.Status_MINED, "test", nil)
		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.processorResponseMap.Len())
	})

	t.Run("SendStatusForTransaction err", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

		throwErr := fmt.Errorf("some error")
		ok, sendErr := processor.SendStatusForTransaction(testdata.TX1Hash, metamorph_api.Status_REJECTED, "test", throwErr)
		assert.True(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_REJECTED, txStored.Status)
		assert.Equal(t, throwErr.Error(), txStored.RejectReason)
	})

	t.Run("SendStatusForTransaction known tx - no update", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(testdata.TX1Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "test", nil)
		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, txStored.Status)
	})

	t.Run("SendStatusForTransaction known tx - processed", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

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

		processor.ProcessTransaction(NewProcessorRequest(
			context.Background(),
			&store.StoreData{
				Hash: testdata.TX1Hash,
			},
			responseChannel,
		))
		wg.Wait()

		assert.Equal(t, 1, processor.processorResponseMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(testdata.TX1Hash, metamorph_api.Status_MINED, "test", nil)
		// need to sleep, since everything is async
		time.Sleep(100 * time.Millisecond)

		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.processorResponseMap.Len(), "should have been removed from the map")

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_MINED, txStored.Status)
	})
}

func TestSendStatusMinedForTransaction(t *testing.T) {
	t.Run("SendStatusMinedForTransaction known tx", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)
		processor.processorResponseMap.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(
			testdata.TX1Hash,
			metamorph_api.Status_SEEN_ON_NETWORK,
		))
		assert.Equal(t, 1, processor.processorResponseMap.Len())

		ok, sendErr := processor.SendStatusMinedForTransaction(testdata.TX1Hash, testdata.Block1Hash, 1233)
		time.Sleep(100 * time.Millisecond)
		assert.True(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

		txStored, err := s.Get(context.Background(), testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_MINED, txStored.Status)
	})

	t.Run("SendStatusMinedForTransaction callback", func(t *testing.T) {
		s, err := sql.New("sqlite_memory")
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

		processor, err := NewProcessor(s, pm, "test", callbackCh, nil)
		require.NoError(t, err)
		// add the tx to the map
		processor.processorResponseMap.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(
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
		s, err := sql.New("sqlite_memory")
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock()

		processor, err := NewProcessor(s, pm, "test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, processor.processorResponseMap.Len())

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

		processor.ProcessTransaction(NewProcessorRequest(
			context.Background(),
			&store.StoreData{
				Hash: testdata.TX1Hash,
			},
			responseChannel,
		))
		wg.Wait()

		assert.Equal(t, 1, processor.processorResponseMap.Len())

		ok, sendErr := processor.SendStatusMinedForTransaction(testdata.TX1Hash, testdata.Block1Hash, 1233)
		time.Sleep(10 * time.Millisecond)
		assert.True(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.processorResponseMap.Len(), "should have been removed from the map")

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
	processor, err := NewProcessor(s, pm, "test", nil, nil)
	require.NoError(b, err)
	processor.SetLogger(slog.New(&TestLogger{}))
	assert.Equal(b, 0, processor.processorResponseMap.Len())

	txs := make(map[string]*chainhash.Hash)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txID := fmt.Sprintf("%x", i)

		txHash := chainhash.HashH([]byte(txID))

		txs[txID] = &txHash

		processor.ProcessTransaction(NewProcessorRequest(
			context.Background(),
			&store.StoreData{
				Hash:   &txHash,
				Status: metamorph_api.Status_UNKNOWN,
				RawTx:  testdata.TX1RawBytes,
			},
			nil,
		))
	}
	b.StopTimer()

	// wait for the last items to be written to the store
	time.Sleep(1 * time.Second)
}

//go:generate moq -pkg mock -out ./store/mock/mock.go ./store/ MetamorphStore
//go:generate moq -pkg mock -out ./blocktx/mock/mock.go ../blocktx/ ClientI
func TestProcessExpiredSeenTransactions(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {

		expectedNrOfUpdates := 3
		expectedNrOfBlockTxRequests := 1
		expectedNumberOfTransactions := 3

		metamorphStore := &storeMock.MetamorphStoreMock{
			UpdateMinedFunc: func(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
				require.Condition(t, func() (success bool) {
					oneOfHash := hash.IsEqual(testdata.TX1Hash) || hash.IsEqual(testdata.TX2Hash) || hash.IsEqual(testdata.TX3Hash)
					isBlockHeight := blockHeight == 1234
					isBlockHash := blockHash.IsEqual(testdata.Block1Hash)
					return oneOfHash && isBlockHeight && isBlockHash
				})

				return nil
			},
		}
		btxMock := &blockTxMock.ClientIMock{
			GetTransactionBlocksFunc: func(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
				require.Equal(t, expectedNumberOfTransactions, len(transaction.Transactions))
				txsBlocks := &blocktx_api.TransactionBlocks{
					TransactionBlocks: []*blocktx_api.TransactionBlock{
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
					},
				}
				return txsBlocks, nil
			},
		}

		pm := p2p.NewPeerManagerMock()
		processor, err := NewProcessor(metamorphStore, pm, "test", nil, btxMock, WithProcessExpiredSeenTxsInterval(20*time.Millisecond))
		require.NoError(t, err)
		defer processor.Shutdown()

		processor.SetLogger(slog.New(&TestLogger{}))
		require.Equal(t, 0, processor.processorResponseMap.Len())

		processor.processorResponseMap.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SEEN_ON_NETWORK))
		processor.processorResponseMap.Set(testdata.TX2Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_SEEN_ON_NETWORK))
		processor.processorResponseMap.Set(testdata.TX3Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX3Hash, metamorph_api.Status_SEEN_ON_NETWORK))

		time.Sleep(50 * time.Millisecond)

		require.Equal(t, expectedNrOfUpdates, len(metamorphStore.UpdateMinedCalls()))
		require.Equal(t, expectedNrOfBlockTxRequests, len(btxMock.GetTransactionBlocksCalls()))
	})
}

type TestLogger struct {
}

func (h *TestLogger) Enabled(_ context.Context, _ slog.Level) bool {
	return false
}

func (h *TestLogger) Handle(_ context.Context, _ slog.Record) error {
	return nil
}

func (h *TestLogger) WithAttrs(_ []slog.Attr) slog.Handler {
	return &TestLogger{}
}

func (h *TestLogger) WithGroup(_ string) slog.Handler {
	return &TestLogger{}
}

func (h *TestLogger) Handler() slog.Handler {
	return &TestLogger{}
}
