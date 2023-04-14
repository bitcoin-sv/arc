package metamorph

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
)

type SendStatusForTransactionCall struct {
	Hash   *chainhash.Hash
	Status metamorph_api.Status
	Err    error
}

type ProcessorMock struct {
	Stats                         *ProcessorStats
	mu                            sync.Mutex
	processTransactionCalls       []*ProcessorRequest
	SendStatusForTransactionCalls []*SendStatusForTransactionCall
}

func NewProcessorMock() *ProcessorMock {
	return &ProcessorMock{
		Stats:                         &ProcessorStats{},
		processTransactionCalls:       make([]*ProcessorRequest, 0),
		SendStatusForTransactionCalls: make([]*SendStatusForTransactionCall, 0),
	}
}

func (p *ProcessorMock) LoadUnmined() {}

func (p *ProcessorMock) GetPeers() ([]string, []string) { return nil, nil }

func (p *ProcessorMock) ProcessTransaction(req *ProcessorRequest) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.processTransactionCalls = append(p.processTransactionCalls, req)
}

func (p *ProcessorMock) GetProcessRequest(index int) *ProcessorRequest {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.processTransactionCalls) <= index {
		return nil
	}

	return p.processTransactionCalls[index]
}

func (p *ProcessorMock) SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, id string, err error) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.SendStatusForTransactionCalls = append(p.SendStatusForTransactionCalls, &SendStatusForTransactionCall{
		Hash:   hash,
		Status: status,
		Err:    err,
	})
	return true, nil
}

func (p *ProcessorMock) SendStatusMinedForTransaction(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.SendStatusForTransactionCalls = append(p.SendStatusForTransactionCalls, &SendStatusForTransactionCall{
		Hash:   hash,
		Status: metamorph_api.Status_MINED,
		Err:    nil,
	})
	return true, nil
}

func (p *ProcessorMock) GetStats(_ bool) *ProcessorStats {
	return p.Stats
}

type BlockTxMock struct {
	mu                                    sync.Mutex
	address                               string
	RegisterTransactionCalls              []*blocktx_api.TransactionAndSource
	RegisterTransactionResponses          []interface{}
	GetBlockCalls                         []*blocktx_api.BlockAndSource
	GetBlockResponses                     []interface{}
	GetMinedTransactionsForBlockCalls     []*blocktx_api.BlockAndSource
	GetMinedTransactionsForBlockResponses []interface{}
}

func NewBlockTxMock(address string) *BlockTxMock {
	return &BlockTxMock{
		mu:                                    sync.Mutex{},
		address:                               address,
		RegisterTransactionCalls:              make([]*blocktx_api.TransactionAndSource, 0),
		RegisterTransactionResponses:          make([]interface{}, 0),
		GetBlockCalls:                         make([]*blocktx_api.BlockAndSource, 0),
		GetBlockResponses:                     make([]interface{}, 0),
		GetMinedTransactionsForBlockCalls:     make([]*blocktx_api.BlockAndSource, 0),
		GetMinedTransactionsForBlockResponses: make([]interface{}, 0),
	}
}

func (b *BlockTxMock) GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.RegisterTransactionResponse, error) {
	// TODO: return mock response
	return nil, nil
}

func (b *BlockTxMock) Start(_ chan *blocktx_api.Block) {
	// we are not starting anything here
}

func (b *BlockTxMock) LocateTransaction(_ context.Context, transaction *blocktx_api.Transaction) (string, error) {
	return b.address, nil
}

func (b *BlockTxMock) RegisterTransaction(_ context.Context, transaction *blocktx_api.TransactionAndSource) (*blocktx_api.RegisterTransactionResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.RegisterTransactionCalls = append(b.RegisterTransactionCalls, transaction)

	if len(b.RegisterTransactionResponses) > 0 {
		resp := b.RegisterTransactionResponses[0]
		switch r := resp.(type) {
		case error:
			return nil, r
		case *blocktx_api.RegisterTransactionResponse:
			return r, nil
		default:
			panic("unknown response type")
		}
	}

	return nil, nil
}

func (b *BlockTxMock) GetBlock(_ context.Context, blockHash *chainhash.Hash) (*blocktx_api.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.GetBlockCalls = append(b.GetBlockCalls, &blocktx_api.BlockAndSource{
		Hash:   blockHash[:],
		Source: b.address,
	})

	if len(b.GetBlockResponses) > 0 {
		resp := b.GetBlockResponses[0]
		switch r := resp.(type) {
		case error:
			return nil, r
		case *blocktx_api.Block:
			return r, nil
		default:
			panic("unknown response type")
		}
	}

	return nil, nil
}

func (b *BlockTxMock) GetMinedTransactionsForBlock(_ context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.GetMinedTransactionsForBlockCalls = append(b.GetMinedTransactionsForBlockCalls, blockAndSource)

	if len(b.GetMinedTransactionsForBlockResponses) > 0 {
		resp := b.GetMinedTransactionsForBlockResponses[0]
		switch r := resp.(type) {
		case error:
			return nil, r
		case *blocktx_api.MinedTransactions:
			return r, nil
		default:
			panic("unknown response type")
		}
	}

	return nil, nil
}

func (b *BlockTxMock) GetLastProcessedBlock(_ context.Context) (*blocktx_api.Block, error) {
	return nil, nil
}

func setStoreTestData(t *testing.T, s store.MetamorphStore) {
	ctx := context.Background()
	err := s.Set(ctx, testdata.TX1Hash[:], &store.StoreData{
		StoredAt:      testdata.Time,
		AnnouncedAt:   testdata.Time.Add(1 * time.Second),
		MinedAt:       testdata.Time.Add(2 * time.Second),
		Hash:          testdata.TX1Hash,
		Status:        metamorph_api.Status_SENT_TO_NETWORK,
		CallbackUrl:   "https://test.com",
		CallbackToken: "token",
	})
	require.NoError(t, err)
	err = s.Set(ctx, testdata.TX2Hash[:], &store.StoreData{
		StoredAt:    testdata.Time,
		AnnouncedAt: testdata.Time.Add(1 * time.Second),
		MinedAt:     testdata.Time.Add(2 * time.Second),
		Hash:        testdata.TX2Hash,
		Status:      metamorph_api.Status_SEEN_ON_NETWORK,
	})
	require.NoError(t, err)
	err = s.Set(ctx, testdata.TX3Hash[:], &store.StoreData{
		StoredAt:    testdata.Time,
		AnnouncedAt: testdata.Time.Add(1 * time.Second),
		MinedAt:     testdata.Time.Add(2 * time.Second),
		Hash:        testdata.TX3Hash,
		Status:      metamorph_api.Status_MINED,
	})
	require.NoError(t, err)
	err = s.Set(ctx, testdata.TX4Hash[:], &store.StoreData{
		StoredAt:    testdata.Time,
		AnnouncedAt: testdata.Time.Add(1 * time.Second),
		MinedAt:     testdata.Time.Add(2 * time.Second),
		Hash:        testdata.TX4Hash,
		Status:      metamorph_api.Status_REJECTED,
	})
	require.NoError(t, err)
}
