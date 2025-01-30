// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"sync"
	"time"
)

// Ensure, that BlocktxStoreMock does implement store.BlocktxStore.
// If this is not the case, regenerate this file with moq.
var _ store.BlocktxStore = &BlocktxStoreMock{}

// BlocktxStoreMock is a mock implementation of store.BlocktxStore.
//
//	func TestSomethingThatUsesBlocktxStore(t *testing.T) {
//
//		// make and configure a mocked store.BlocktxStore
//		mockedBlocktxStore := &BlocktxStoreMock{
//			ClearBlocktxTableFunc: func(ctx context.Context, retentionDays int32, table string) (*blocktx_api.RowsAffectedResponse, error) {
//				panic("mock out the ClearBlocktxTable method")
//			},
//			CloseFunc: func() error {
//				panic("mock out the Close method")
//			},
//			GetBlockFunc: func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
//				panic("mock out the GetBlock method")
//			},
//			GetBlockGapsFunc: func(ctx context.Context, heightRange int) ([]*store.BlockGap, error) {
//				panic("mock out the GetBlockGaps method")
//			},
//			GetBlockTransactionsHashesFunc: func(ctx context.Context, blockHash []byte) ([]*chainhash.Hash, error) {
//				panic("mock out the GetBlockTransactionsHashes method")
//			},
//			GetChainTipFunc: func(ctx context.Context, heightRange int) (*blocktx_api.Block, error) {
//				panic("mock out the GetChainTip method")
//			},
//			GetLongestBlockByHeightFunc: func(ctx context.Context, height uint64) (*blocktx_api.Block, error) {
//				panic("mock out the GetLongestBlockByHeight method")
//			},
//			GetLongestChainFromHeightFunc: func(ctx context.Context, height uint64) ([]*blocktx_api.Block, error) {
//				panic("mock out the GetLongestChainFromHeight method")
//			},
//			GetMinedTransactionsFunc: func(ctx context.Context, hashes [][]byte) ([]store.BlockTransaction, error) {
//				panic("mock out the GetMinedTransactions method")
//			},
//			GetOrphansBackToNonOrphanAncestorFunc: func(ctx context.Context, hash []byte) ([]*blocktx_api.Block, *blocktx_api.Block, error) {
//				panic("mock out the GetOrphansBackToNonOrphanAncestor method")
//			},
//			GetRegisteredTxsByBlockHashesFunc: func(ctx context.Context, blockHashes [][]byte) ([]store.BlockTransaction, error) {
//				panic("mock out the GetRegisteredTxsByBlockHashes method")
//			},
//			GetStaleChainBackFromHashFunc: func(ctx context.Context, hash []byte) ([]*blocktx_api.Block, error) {
//				panic("mock out the GetStaleChainBackFromHash method")
//			},
//			GetStatsFunc: func(ctx context.Context) (*store.Stats, error) {
//				panic("mock out the GetStats method")
//			},
//			InsertBlockTransactionsFunc: func(ctx context.Context, blockID uint64, txsWithMerklePaths []store.TxHashWithMerkleTreeIndex) error {
//				panic("mock out the InsertBlockTransactions method")
//			},
//			MarkBlockAsDoneFunc: func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
//				panic("mock out the MarkBlockAsDone method")
//			},
//			PingFunc: func(ctx context.Context) error {
//				panic("mock out the Ping method")
//			},
//			RegisterTransactionsFunc: func(ctx context.Context, txHashes [][]byte) (int64, error) {
//				panic("mock out the RegisterTransactions method")
//			},
//			SetBlockProcessingFunc: func(ctx context.Context, hash *chainhash.Hash, setProcessedBy string, lockTime time.Duration, maxParallelProcessing int) (string, error) {
//				panic("mock out the SetBlockProcessing method")
//			},
//			UpdateBlocksStatusesFunc: func(ctx context.Context, blockStatusUpdates []store.BlockStatusUpdate) error {
//				panic("mock out the UpdateBlocksStatuses method")
//			},
//			UpsertBlockFunc: func(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
//				panic("mock out the UpsertBlock method")
//			},
//			VerifyMerkleRootsFunc: func(ctx context.Context, merkleRoots []*blocktx_api.MerkleRootVerificationRequest, maxAllowedBlockHeightMismatch int) (*blocktx_api.MerkleRootVerificationResponse, error) {
//				panic("mock out the VerifyMerkleRoots method")
//			},
//		}
//
//		// use mockedBlocktxStore in code that requires store.BlocktxStore
//		// and then make assertions.
//
//	}
type BlocktxStoreMock struct {
	// ClearBlocktxTableFunc mocks the ClearBlocktxTable method.
	ClearBlocktxTableFunc func(ctx context.Context, retentionDays int32, table string) (*blocktx_api.RowsAffectedResponse, error)

	// CloseFunc mocks the Close method.
	CloseFunc func() error

	// GetBlockFunc mocks the GetBlock method.
	GetBlockFunc func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error)

	// GetBlockGapsFunc mocks the GetBlockGaps method.
	GetBlockGapsFunc func(ctx context.Context, heightRange int) ([]*store.BlockGap, error)

	// GetBlockTransactionsHashesFunc mocks the GetBlockTransactionsHashes method.
	GetBlockTransactionsHashesFunc func(ctx context.Context, blockHash []byte) ([]*chainhash.Hash, error)

	// GetChainTipFunc mocks the GetChainTip method.
	GetChainTipFunc func(ctx context.Context, heightRange int) (*blocktx_api.Block, error)

	// GetLongestBlockByHeightFunc mocks the GetLongestBlockByHeight method.
	GetLongestBlockByHeightFunc func(ctx context.Context, height uint64) (*blocktx_api.Block, error)

	// GetLongestChainFromHeightFunc mocks the GetLongestChainFromHeight method.
	GetLongestChainFromHeightFunc func(ctx context.Context, height uint64) ([]*blocktx_api.Block, error)

	// GetMinedTransactionsFunc mocks the GetMinedTransactions method.
	GetMinedTransactionsFunc func(ctx context.Context, hashes [][]byte) ([]store.BlockTransaction, error)

	// GetOrphansBackToNonOrphanAncestorFunc mocks the GetOrphansBackToNonOrphanAncestor method.
	GetOrphansBackToNonOrphanAncestorFunc func(ctx context.Context, hash []byte) ([]*blocktx_api.Block, *blocktx_api.Block, error)

	// GetRegisteredTxsByBlockHashesFunc mocks the GetRegisteredTxsByBlockHashes method.
	GetRegisteredTxsByBlockHashesFunc func(ctx context.Context, blockHashes [][]byte) ([]store.BlockTransaction, error)

	// GetStaleChainBackFromHashFunc mocks the GetStaleChainBackFromHash method.
	GetStaleChainBackFromHashFunc func(ctx context.Context, hash []byte) ([]*blocktx_api.Block, error)

	// GetStatsFunc mocks the GetStats method.
	GetStatsFunc func(ctx context.Context) (*store.Stats, error)

	// InsertBlockTransactionsFunc mocks the InsertBlockTransactions method.
	InsertBlockTransactionsFunc func(ctx context.Context, blockID uint64, txsWithMerklePaths []store.TxHashWithMerkleTreeIndex) error

	// MarkBlockAsDoneFunc mocks the MarkBlockAsDone method.
	MarkBlockAsDoneFunc func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error

	// PingFunc mocks the Ping method.
	PingFunc func(ctx context.Context) error

	// RegisterTransactionsFunc mocks the RegisterTransactions method.
	RegisterTransactionsFunc func(ctx context.Context, txHashes [][]byte) (int64, error)

	// SetBlockProcessingFunc mocks the SetBlockProcessing method.
	SetBlockProcessingFunc func(ctx context.Context, hash *chainhash.Hash, setProcessedBy string, lockTime time.Duration, maxParallelProcessing int) (string, error)

	// UpdateBlocksStatusesFunc mocks the UpdateBlocksStatuses method.
	UpdateBlocksStatusesFunc func(ctx context.Context, blockStatusUpdates []store.BlockStatusUpdate) error

	// UpsertBlockFunc mocks the UpsertBlock method.
	UpsertBlockFunc func(ctx context.Context, block *blocktx_api.Block) (uint64, error)

	// VerifyMerkleRootsFunc mocks the VerifyMerkleRoots method.
	VerifyMerkleRootsFunc func(ctx context.Context, merkleRoots []*blocktx_api.MerkleRootVerificationRequest, maxAllowedBlockHeightMismatch int) (*blocktx_api.MerkleRootVerificationResponse, error)

	// calls tracks calls to the methods.
	calls struct {
		// ClearBlocktxTable holds details about calls to the ClearBlocktxTable method.
		ClearBlocktxTable []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// RetentionDays is the retentionDays argument value.
			RetentionDays int32
			// Table is the table argument value.
			Table string
		}
		// Close holds details about calls to the Close method.
		Close []struct {
		}
		// GetBlock holds details about calls to the GetBlock method.
		GetBlock []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
		}
		// GetBlockGaps holds details about calls to the GetBlockGaps method.
		GetBlockGaps []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// HeightRange is the heightRange argument value.
			HeightRange int
		}
		// GetBlockTransactionsHashes holds details about calls to the GetBlockTransactionsHashes method.
		GetBlockTransactionsHashes []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockHash is the blockHash argument value.
			BlockHash []byte
		}
		// GetChainTip holds details about calls to the GetChainTip method.
		GetChainTip []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// HeightRange is the heightRange argument value.
			HeightRange int
		}
		// GetLongestBlockByHeight holds details about calls to the GetLongestBlockByHeight method.
		GetLongestBlockByHeight []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Height is the height argument value.
			Height uint64
		}
		// GetLongestChainFromHeight holds details about calls to the GetLongestChainFromHeight method.
		GetLongestChainFromHeight []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Height is the height argument value.
			Height uint64
		}
		// GetMinedTransactions holds details about calls to the GetMinedTransactions method.
		GetMinedTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hashes is the hashes argument value.
			Hashes [][]byte
		}
		// GetOrphansBackToNonOrphanAncestor holds details about calls to the GetOrphansBackToNonOrphanAncestor method.
		GetOrphansBackToNonOrphanAncestor []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash []byte
		}
		// GetRegisteredTxsByBlockHashes holds details about calls to the GetRegisteredTxsByBlockHashes method.
		GetRegisteredTxsByBlockHashes []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockHashes is the blockHashes argument value.
			BlockHashes [][]byte
		}
		// GetStaleChainBackFromHash holds details about calls to the GetStaleChainBackFromHash method.
		GetStaleChainBackFromHash []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash []byte
		}
		// GetStats holds details about calls to the GetStats method.
		GetStats []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// InsertBlockTransactions holds details about calls to the InsertBlockTransactions method.
		InsertBlockTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockID is the blockID argument value.
			BlockID uint64
			// TxsWithMerklePaths is the txsWithMerklePaths argument value.
			TxsWithMerklePaths []store.TxHashWithMerkleTreeIndex
		}
		// MarkBlockAsDone holds details about calls to the MarkBlockAsDone method.
		MarkBlockAsDone []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
			// Size is the size argument value.
			Size uint64
			// TxCount is the txCount argument value.
			TxCount uint64
		}
		// Ping holds details about calls to the Ping method.
		Ping []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// RegisterTransactions holds details about calls to the RegisterTransactions method.
		RegisterTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// TxHashes is the txHashes argument value.
			TxHashes [][]byte
		}
		// SetBlockProcessing holds details about calls to the SetBlockProcessing method.
		SetBlockProcessing []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
			// SetProcessedBy is the setProcessedBy argument value.
			SetProcessedBy string
			// LockTime is the lockTime argument value.
			LockTime time.Duration
			// MaxParallelProcessing is the maxParallelProcessing argument value.
			MaxParallelProcessing int
		}
		// UpdateBlocksStatuses holds details about calls to the UpdateBlocksStatuses method.
		UpdateBlocksStatuses []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockStatusUpdates is the blockStatusUpdates argument value.
			BlockStatusUpdates []store.BlockStatusUpdate
		}
		// UpsertBlock holds details about calls to the UpsertBlock method.
		UpsertBlock []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Block is the block argument value.
			Block *blocktx_api.Block
		}
		// VerifyMerkleRoots holds details about calls to the VerifyMerkleRoots method.
		VerifyMerkleRoots []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// MerkleRoots is the merkleRoots argument value.
			MerkleRoots []*blocktx_api.MerkleRootVerificationRequest
			// MaxAllowedBlockHeightMismatch is the maxAllowedBlockHeightMismatch argument value.
			MaxAllowedBlockHeightMismatch int
		}
	}
	lockClearBlocktxTable                 sync.RWMutex
	lockClose                             sync.RWMutex
	lockGetBlock                          sync.RWMutex
	lockGetBlockGaps                      sync.RWMutex
	lockGetBlockTransactionsHashes        sync.RWMutex
	lockGetChainTip                       sync.RWMutex
	lockGetLongestBlockByHeight           sync.RWMutex
	lockGetLongestChainFromHeight         sync.RWMutex
	lockGetMinedTransactions              sync.RWMutex
	lockGetOrphansBackToNonOrphanAncestor sync.RWMutex
	lockGetRegisteredTxsByBlockHashes     sync.RWMutex
	lockGetStaleChainBackFromHash         sync.RWMutex
	lockGetStats                          sync.RWMutex
	lockInsertBlockTransactions           sync.RWMutex
	lockMarkBlockAsDone                   sync.RWMutex
	lockPing                              sync.RWMutex
	lockRegisterTransactions              sync.RWMutex
	lockSetBlockProcessing                sync.RWMutex
	lockUpdateBlocksStatuses              sync.RWMutex
	lockUpsertBlock                       sync.RWMutex
	lockVerifyMerkleRoots                 sync.RWMutex
}

// ClearBlocktxTable calls ClearBlocktxTableFunc.
func (mock *BlocktxStoreMock) ClearBlocktxTable(ctx context.Context, retentionDays int32, table string) (*blocktx_api.RowsAffectedResponse, error) {
	if mock.ClearBlocktxTableFunc == nil {
		panic("BlocktxStoreMock.ClearBlocktxTableFunc: method is nil but BlocktxStore.ClearBlocktxTable was just called")
	}
	callInfo := struct {
		Ctx           context.Context
		RetentionDays int32
		Table         string
	}{
		Ctx:           ctx,
		RetentionDays: retentionDays,
		Table:         table,
	}
	mock.lockClearBlocktxTable.Lock()
	mock.calls.ClearBlocktxTable = append(mock.calls.ClearBlocktxTable, callInfo)
	mock.lockClearBlocktxTable.Unlock()
	return mock.ClearBlocktxTableFunc(ctx, retentionDays, table)
}

// ClearBlocktxTableCalls gets all the calls that were made to ClearBlocktxTable.
// Check the length with:
//
//	len(mockedBlocktxStore.ClearBlocktxTableCalls())
func (mock *BlocktxStoreMock) ClearBlocktxTableCalls() []struct {
	Ctx           context.Context
	RetentionDays int32
	Table         string
} {
	var calls []struct {
		Ctx           context.Context
		RetentionDays int32
		Table         string
	}
	mock.lockClearBlocktxTable.RLock()
	calls = mock.calls.ClearBlocktxTable
	mock.lockClearBlocktxTable.RUnlock()
	return calls
}

// Close calls CloseFunc.
func (mock *BlocktxStoreMock) Close() error {
	if mock.CloseFunc == nil {
		panic("BlocktxStoreMock.CloseFunc: method is nil but BlocktxStore.Close was just called")
	}
	callInfo := struct {
	}{}
	mock.lockClose.Lock()
	mock.calls.Close = append(mock.calls.Close, callInfo)
	mock.lockClose.Unlock()
	return mock.CloseFunc()
}

// CloseCalls gets all the calls that were made to Close.
// Check the length with:
//
//	len(mockedBlocktxStore.CloseCalls())
func (mock *BlocktxStoreMock) CloseCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockClose.RLock()
	calls = mock.calls.Close
	mock.lockClose.RUnlock()
	return calls
}

// GetBlock calls GetBlockFunc.
func (mock *BlocktxStoreMock) GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
	if mock.GetBlockFunc == nil {
		panic("BlocktxStoreMock.GetBlockFunc: method is nil but BlocktxStore.GetBlock was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Hash *chainhash.Hash
	}{
		Ctx:  ctx,
		Hash: hash,
	}
	mock.lockGetBlock.Lock()
	mock.calls.GetBlock = append(mock.calls.GetBlock, callInfo)
	mock.lockGetBlock.Unlock()
	return mock.GetBlockFunc(ctx, hash)
}

// GetBlockCalls gets all the calls that were made to GetBlock.
// Check the length with:
//
//	len(mockedBlocktxStore.GetBlockCalls())
func (mock *BlocktxStoreMock) GetBlockCalls() []struct {
	Ctx  context.Context
	Hash *chainhash.Hash
} {
	var calls []struct {
		Ctx  context.Context
		Hash *chainhash.Hash
	}
	mock.lockGetBlock.RLock()
	calls = mock.calls.GetBlock
	mock.lockGetBlock.RUnlock()
	return calls
}

// GetBlockGaps calls GetBlockGapsFunc.
func (mock *BlocktxStoreMock) GetBlockGaps(ctx context.Context, heightRange int) ([]*store.BlockGap, error) {
	if mock.GetBlockGapsFunc == nil {
		panic("BlocktxStoreMock.GetBlockGapsFunc: method is nil but BlocktxStore.GetBlockGaps was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		HeightRange int
	}{
		Ctx:         ctx,
		HeightRange: heightRange,
	}
	mock.lockGetBlockGaps.Lock()
	mock.calls.GetBlockGaps = append(mock.calls.GetBlockGaps, callInfo)
	mock.lockGetBlockGaps.Unlock()
	return mock.GetBlockGapsFunc(ctx, heightRange)
}

// GetBlockGapsCalls gets all the calls that were made to GetBlockGaps.
// Check the length with:
//
//	len(mockedBlocktxStore.GetBlockGapsCalls())
func (mock *BlocktxStoreMock) GetBlockGapsCalls() []struct {
	Ctx         context.Context
	HeightRange int
} {
	var calls []struct {
		Ctx         context.Context
		HeightRange int
	}
	mock.lockGetBlockGaps.RLock()
	calls = mock.calls.GetBlockGaps
	mock.lockGetBlockGaps.RUnlock()
	return calls
}

// GetBlockTransactionsHashes calls GetBlockTransactionsHashesFunc.
func (mock *BlocktxStoreMock) GetBlockTransactionsHashes(ctx context.Context, blockHash []byte) ([]*chainhash.Hash, error) {
	if mock.GetBlockTransactionsHashesFunc == nil {
		panic("BlocktxStoreMock.GetBlockTransactionsHashesFunc: method is nil but BlocktxStore.GetBlockTransactionsHashes was just called")
	}
	callInfo := struct {
		Ctx       context.Context
		BlockHash []byte
	}{
		Ctx:       ctx,
		BlockHash: blockHash,
	}
	mock.lockGetBlockTransactionsHashes.Lock()
	mock.calls.GetBlockTransactionsHashes = append(mock.calls.GetBlockTransactionsHashes, callInfo)
	mock.lockGetBlockTransactionsHashes.Unlock()
	return mock.GetBlockTransactionsHashesFunc(ctx, blockHash)
}

// GetBlockTransactionsHashesCalls gets all the calls that were made to GetBlockTransactionsHashes.
// Check the length with:
//
//	len(mockedBlocktxStore.GetBlockTransactionsHashesCalls())
func (mock *BlocktxStoreMock) GetBlockTransactionsHashesCalls() []struct {
	Ctx       context.Context
	BlockHash []byte
} {
	var calls []struct {
		Ctx       context.Context
		BlockHash []byte
	}
	mock.lockGetBlockTransactionsHashes.RLock()
	calls = mock.calls.GetBlockTransactionsHashes
	mock.lockGetBlockTransactionsHashes.RUnlock()
	return calls
}

// GetChainTip calls GetChainTipFunc.
func (mock *BlocktxStoreMock) GetChainTip(ctx context.Context, heightRange int) (*blocktx_api.Block, error) {
	if mock.GetChainTipFunc == nil {
		panic("BlocktxStoreMock.GetChainTipFunc: method is nil but BlocktxStore.GetChainTip was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		HeightRange int
	}{
		Ctx:         ctx,
		HeightRange: heightRange,
	}
	mock.lockGetChainTip.Lock()
	mock.calls.GetChainTip = append(mock.calls.GetChainTip, callInfo)
	mock.lockGetChainTip.Unlock()
	return mock.GetChainTipFunc(ctx, heightRange)
}

// GetChainTipCalls gets all the calls that were made to GetChainTip.
// Check the length with:
//
//	len(mockedBlocktxStore.GetChainTipCalls())
func (mock *BlocktxStoreMock) GetChainTipCalls() []struct {
	Ctx         context.Context
	HeightRange int
} {
	var calls []struct {
		Ctx         context.Context
		HeightRange int
	}
	mock.lockGetChainTip.RLock()
	calls = mock.calls.GetChainTip
	mock.lockGetChainTip.RUnlock()
	return calls
}

// GetLongestBlockByHeight calls GetLongestBlockByHeightFunc.
func (mock *BlocktxStoreMock) GetLongestBlockByHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error) {
	if mock.GetLongestBlockByHeightFunc == nil {
		panic("BlocktxStoreMock.GetLongestBlockByHeightFunc: method is nil but BlocktxStore.GetLongestBlockByHeight was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Height uint64
	}{
		Ctx:    ctx,
		Height: height,
	}
	mock.lockGetLongestBlockByHeight.Lock()
	mock.calls.GetLongestBlockByHeight = append(mock.calls.GetLongestBlockByHeight, callInfo)
	mock.lockGetLongestBlockByHeight.Unlock()
	return mock.GetLongestBlockByHeightFunc(ctx, height)
}

// GetLongestBlockByHeightCalls gets all the calls that were made to GetLongestBlockByHeight.
// Check the length with:
//
//	len(mockedBlocktxStore.GetLongestBlockByHeightCalls())
func (mock *BlocktxStoreMock) GetLongestBlockByHeightCalls() []struct {
	Ctx    context.Context
	Height uint64
} {
	var calls []struct {
		Ctx    context.Context
		Height uint64
	}
	mock.lockGetLongestBlockByHeight.RLock()
	calls = mock.calls.GetLongestBlockByHeight
	mock.lockGetLongestBlockByHeight.RUnlock()
	return calls
}

// GetLongestChainFromHeight calls GetLongestChainFromHeightFunc.
func (mock *BlocktxStoreMock) GetLongestChainFromHeight(ctx context.Context, height uint64) ([]*blocktx_api.Block, error) {
	if mock.GetLongestChainFromHeightFunc == nil {
		panic("BlocktxStoreMock.GetLongestChainFromHeightFunc: method is nil but BlocktxStore.GetLongestChainFromHeight was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Height uint64
	}{
		Ctx:    ctx,
		Height: height,
	}
	mock.lockGetLongestChainFromHeight.Lock()
	mock.calls.GetLongestChainFromHeight = append(mock.calls.GetLongestChainFromHeight, callInfo)
	mock.lockGetLongestChainFromHeight.Unlock()
	return mock.GetLongestChainFromHeightFunc(ctx, height)
}

// GetLongestChainFromHeightCalls gets all the calls that were made to GetLongestChainFromHeight.
// Check the length with:
//
//	len(mockedBlocktxStore.GetLongestChainFromHeightCalls())
func (mock *BlocktxStoreMock) GetLongestChainFromHeightCalls() []struct {
	Ctx    context.Context
	Height uint64
} {
	var calls []struct {
		Ctx    context.Context
		Height uint64
	}
	mock.lockGetLongestChainFromHeight.RLock()
	calls = mock.calls.GetLongestChainFromHeight
	mock.lockGetLongestChainFromHeight.RUnlock()
	return calls
}

// GetMinedTransactions calls GetMinedTransactionsFunc.
func (mock *BlocktxStoreMock) GetMinedTransactions(ctx context.Context, hashes [][]byte) ([]store.BlockTransaction, error) {
	if mock.GetMinedTransactionsFunc == nil {
		panic("BlocktxStoreMock.GetMinedTransactionsFunc: method is nil but BlocktxStore.GetMinedTransactions was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Hashes [][]byte
	}{
		Ctx:    ctx,
		Hashes: hashes,
	}
	mock.lockGetMinedTransactions.Lock()
	mock.calls.GetMinedTransactions = append(mock.calls.GetMinedTransactions, callInfo)
	mock.lockGetMinedTransactions.Unlock()
	return mock.GetMinedTransactionsFunc(ctx, hashes)
}

// GetMinedTransactionsCalls gets all the calls that were made to GetMinedTransactions.
// Check the length with:
//
//	len(mockedBlocktxStore.GetMinedTransactionsCalls())
func (mock *BlocktxStoreMock) GetMinedTransactionsCalls() []struct {
	Ctx    context.Context
	Hashes [][]byte
} {
	var calls []struct {
		Ctx    context.Context
		Hashes [][]byte
	}
	mock.lockGetMinedTransactions.RLock()
	calls = mock.calls.GetMinedTransactions
	mock.lockGetMinedTransactions.RUnlock()
	return calls
}

// GetOrphansBackToNonOrphanAncestor calls GetOrphansBackToNonOrphanAncestorFunc.
func (mock *BlocktxStoreMock) GetOrphansBackToNonOrphanAncestor(ctx context.Context, hash []byte) ([]*blocktx_api.Block, *blocktx_api.Block, error) {
	if mock.GetOrphansBackToNonOrphanAncestorFunc == nil {
		panic("BlocktxStoreMock.GetOrphansBackToNonOrphanAncestorFunc: method is nil but BlocktxStore.GetOrphansBackToNonOrphanAncestor was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Hash []byte
	}{
		Ctx:  ctx,
		Hash: hash,
	}
	mock.lockGetOrphansBackToNonOrphanAncestor.Lock()
	mock.calls.GetOrphansBackToNonOrphanAncestor = append(mock.calls.GetOrphansBackToNonOrphanAncestor, callInfo)
	mock.lockGetOrphansBackToNonOrphanAncestor.Unlock()
	return mock.GetOrphansBackToNonOrphanAncestorFunc(ctx, hash)
}

// GetOrphansBackToNonOrphanAncestorCalls gets all the calls that were made to GetOrphansBackToNonOrphanAncestor.
// Check the length with:
//
//	len(mockedBlocktxStore.GetOrphansBackToNonOrphanAncestorCalls())
func (mock *BlocktxStoreMock) GetOrphansBackToNonOrphanAncestorCalls() []struct {
	Ctx  context.Context
	Hash []byte
} {
	var calls []struct {
		Ctx  context.Context
		Hash []byte
	}
	mock.lockGetOrphansBackToNonOrphanAncestor.RLock()
	calls = mock.calls.GetOrphansBackToNonOrphanAncestor
	mock.lockGetOrphansBackToNonOrphanAncestor.RUnlock()
	return calls
}

// GetRegisteredTxsByBlockHashes calls GetRegisteredTxsByBlockHashesFunc.
func (mock *BlocktxStoreMock) GetRegisteredTxsByBlockHashes(ctx context.Context, blockHashes [][]byte) ([]store.BlockTransaction, error) {
	if mock.GetRegisteredTxsByBlockHashesFunc == nil {
		panic("BlocktxStoreMock.GetRegisteredTxsByBlockHashesFunc: method is nil but BlocktxStore.GetRegisteredTxsByBlockHashes was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		BlockHashes [][]byte
	}{
		Ctx:         ctx,
		BlockHashes: blockHashes,
	}
	mock.lockGetRegisteredTxsByBlockHashes.Lock()
	mock.calls.GetRegisteredTxsByBlockHashes = append(mock.calls.GetRegisteredTxsByBlockHashes, callInfo)
	mock.lockGetRegisteredTxsByBlockHashes.Unlock()
	return mock.GetRegisteredTxsByBlockHashesFunc(ctx, blockHashes)
}

// GetRegisteredTxsByBlockHashesCalls gets all the calls that were made to GetRegisteredTxsByBlockHashes.
// Check the length with:
//
//	len(mockedBlocktxStore.GetRegisteredTxsByBlockHashesCalls())
func (mock *BlocktxStoreMock) GetRegisteredTxsByBlockHashesCalls() []struct {
	Ctx         context.Context
	BlockHashes [][]byte
} {
	var calls []struct {
		Ctx         context.Context
		BlockHashes [][]byte
	}
	mock.lockGetRegisteredTxsByBlockHashes.RLock()
	calls = mock.calls.GetRegisteredTxsByBlockHashes
	mock.lockGetRegisteredTxsByBlockHashes.RUnlock()
	return calls
}

// GetStaleChainBackFromHash calls GetStaleChainBackFromHashFunc.
func (mock *BlocktxStoreMock) GetStaleChainBackFromHash(ctx context.Context, hash []byte) ([]*blocktx_api.Block, error) {
	if mock.GetStaleChainBackFromHashFunc == nil {
		panic("BlocktxStoreMock.GetStaleChainBackFromHashFunc: method is nil but BlocktxStore.GetStaleChainBackFromHash was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Hash []byte
	}{
		Ctx:  ctx,
		Hash: hash,
	}
	mock.lockGetStaleChainBackFromHash.Lock()
	mock.calls.GetStaleChainBackFromHash = append(mock.calls.GetStaleChainBackFromHash, callInfo)
	mock.lockGetStaleChainBackFromHash.Unlock()
	return mock.GetStaleChainBackFromHashFunc(ctx, hash)
}

// GetStaleChainBackFromHashCalls gets all the calls that were made to GetStaleChainBackFromHash.
// Check the length with:
//
//	len(mockedBlocktxStore.GetStaleChainBackFromHashCalls())
func (mock *BlocktxStoreMock) GetStaleChainBackFromHashCalls() []struct {
	Ctx  context.Context
	Hash []byte
} {
	var calls []struct {
		Ctx  context.Context
		Hash []byte
	}
	mock.lockGetStaleChainBackFromHash.RLock()
	calls = mock.calls.GetStaleChainBackFromHash
	mock.lockGetStaleChainBackFromHash.RUnlock()
	return calls
}

// GetStats calls GetStatsFunc.
func (mock *BlocktxStoreMock) GetStats(ctx context.Context) (*store.Stats, error) {
	if mock.GetStatsFunc == nil {
		panic("BlocktxStoreMock.GetStatsFunc: method is nil but BlocktxStore.GetStats was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetStats.Lock()
	mock.calls.GetStats = append(mock.calls.GetStats, callInfo)
	mock.lockGetStats.Unlock()
	return mock.GetStatsFunc(ctx)
}

// GetStatsCalls gets all the calls that were made to GetStats.
// Check the length with:
//
//	len(mockedBlocktxStore.GetStatsCalls())
func (mock *BlocktxStoreMock) GetStatsCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetStats.RLock()
	calls = mock.calls.GetStats
	mock.lockGetStats.RUnlock()
	return calls
}

// InsertBlockTransactions calls InsertBlockTransactionsFunc.
func (mock *BlocktxStoreMock) InsertBlockTransactions(ctx context.Context, blockID uint64, txsWithMerklePaths []store.TxHashWithMerkleTreeIndex) error {
	if mock.InsertBlockTransactionsFunc == nil {
		panic("BlocktxStoreMock.InsertBlockTransactionsFunc: method is nil but BlocktxStore.InsertBlockTransactions was just called")
	}
	callInfo := struct {
		Ctx                context.Context
		BlockID            uint64
		TxsWithMerklePaths []store.TxHashWithMerkleTreeIndex
	}{
		Ctx:                ctx,
		BlockID:            blockID,
		TxsWithMerklePaths: txsWithMerklePaths,
	}
	mock.lockInsertBlockTransactions.Lock()
	mock.calls.InsertBlockTransactions = append(mock.calls.InsertBlockTransactions, callInfo)
	mock.lockInsertBlockTransactions.Unlock()
	return mock.InsertBlockTransactionsFunc(ctx, blockID, txsWithMerklePaths)
}

// InsertBlockTransactionsCalls gets all the calls that were made to InsertBlockTransactions.
// Check the length with:
//
//	len(mockedBlocktxStore.InsertBlockTransactionsCalls())
func (mock *BlocktxStoreMock) InsertBlockTransactionsCalls() []struct {
	Ctx                context.Context
	BlockID            uint64
	TxsWithMerklePaths []store.TxHashWithMerkleTreeIndex
} {
	var calls []struct {
		Ctx                context.Context
		BlockID            uint64
		TxsWithMerklePaths []store.TxHashWithMerkleTreeIndex
	}
	mock.lockInsertBlockTransactions.RLock()
	calls = mock.calls.InsertBlockTransactions
	mock.lockInsertBlockTransactions.RUnlock()
	return calls
}

// MarkBlockAsDone calls MarkBlockAsDoneFunc.
func (mock *BlocktxStoreMock) MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
	if mock.MarkBlockAsDoneFunc == nil {
		panic("BlocktxStoreMock.MarkBlockAsDoneFunc: method is nil but BlocktxStore.MarkBlockAsDone was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Hash    *chainhash.Hash
		Size    uint64
		TxCount uint64
	}{
		Ctx:     ctx,
		Hash:    hash,
		Size:    size,
		TxCount: txCount,
	}
	mock.lockMarkBlockAsDone.Lock()
	mock.calls.MarkBlockAsDone = append(mock.calls.MarkBlockAsDone, callInfo)
	mock.lockMarkBlockAsDone.Unlock()
	return mock.MarkBlockAsDoneFunc(ctx, hash, size, txCount)
}

// MarkBlockAsDoneCalls gets all the calls that were made to MarkBlockAsDone.
// Check the length with:
//
//	len(mockedBlocktxStore.MarkBlockAsDoneCalls())
func (mock *BlocktxStoreMock) MarkBlockAsDoneCalls() []struct {
	Ctx     context.Context
	Hash    *chainhash.Hash
	Size    uint64
	TxCount uint64
} {
	var calls []struct {
		Ctx     context.Context
		Hash    *chainhash.Hash
		Size    uint64
		TxCount uint64
	}
	mock.lockMarkBlockAsDone.RLock()
	calls = mock.calls.MarkBlockAsDone
	mock.lockMarkBlockAsDone.RUnlock()
	return calls
}

// Ping calls PingFunc.
func (mock *BlocktxStoreMock) Ping(ctx context.Context) error {
	if mock.PingFunc == nil {
		panic("BlocktxStoreMock.PingFunc: method is nil but BlocktxStore.Ping was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockPing.Lock()
	mock.calls.Ping = append(mock.calls.Ping, callInfo)
	mock.lockPing.Unlock()
	return mock.PingFunc(ctx)
}

// PingCalls gets all the calls that were made to Ping.
// Check the length with:
//
//	len(mockedBlocktxStore.PingCalls())
func (mock *BlocktxStoreMock) PingCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockPing.RLock()
	calls = mock.calls.Ping
	mock.lockPing.RUnlock()
	return calls
}

// RegisterTransactions calls RegisterTransactionsFunc.
func (mock *BlocktxStoreMock) RegisterTransactions(ctx context.Context, txHashes [][]byte) (int64, error) {
	if mock.RegisterTransactionsFunc == nil {
		panic("BlocktxStoreMock.RegisterTransactionsFunc: method is nil but BlocktxStore.RegisterTransactions was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		TxHashes [][]byte
	}{
		Ctx:      ctx,
		TxHashes: txHashes,
	}
	mock.lockRegisterTransactions.Lock()
	mock.calls.RegisterTransactions = append(mock.calls.RegisterTransactions, callInfo)
	mock.lockRegisterTransactions.Unlock()
	return mock.RegisterTransactionsFunc(ctx, txHashes)
}

// RegisterTransactionsCalls gets all the calls that were made to RegisterTransactions.
// Check the length with:
//
//	len(mockedBlocktxStore.RegisterTransactionsCalls())
func (mock *BlocktxStoreMock) RegisterTransactionsCalls() []struct {
	Ctx      context.Context
	TxHashes [][]byte
} {
	var calls []struct {
		Ctx      context.Context
		TxHashes [][]byte
	}
	mock.lockRegisterTransactions.RLock()
	calls = mock.calls.RegisterTransactions
	mock.lockRegisterTransactions.RUnlock()
	return calls
}

// SetBlockProcessing calls SetBlockProcessingFunc.
func (mock *BlocktxStoreMock) SetBlockProcessing(ctx context.Context, hash *chainhash.Hash, setProcessedBy string, lockTime time.Duration, maxParallelProcessing int) (string, error) {
	if mock.SetBlockProcessingFunc == nil {
		panic("BlocktxStoreMock.SetBlockProcessingFunc: method is nil but BlocktxStore.SetBlockProcessing was just called")
	}
	callInfo := struct {
		Ctx                   context.Context
		Hash                  *chainhash.Hash
		SetProcessedBy        string
		LockTime              time.Duration
		MaxParallelProcessing int
	}{
		Ctx:                   ctx,
		Hash:                  hash,
		SetProcessedBy:        setProcessedBy,
		LockTime:              lockTime,
		MaxParallelProcessing: maxParallelProcessing,
	}
	mock.lockSetBlockProcessing.Lock()
	mock.calls.SetBlockProcessing = append(mock.calls.SetBlockProcessing, callInfo)
	mock.lockSetBlockProcessing.Unlock()
	return mock.SetBlockProcessingFunc(ctx, hash, setProcessedBy, lockTime, maxParallelProcessing)
}

// SetBlockProcessingCalls gets all the calls that were made to SetBlockProcessing.
// Check the length with:
//
//	len(mockedBlocktxStore.SetBlockProcessingCalls())
func (mock *BlocktxStoreMock) SetBlockProcessingCalls() []struct {
	Ctx                   context.Context
	Hash                  *chainhash.Hash
	SetProcessedBy        string
	LockTime              time.Duration
	MaxParallelProcessing int
} {
	var calls []struct {
		Ctx                   context.Context
		Hash                  *chainhash.Hash
		SetProcessedBy        string
		LockTime              time.Duration
		MaxParallelProcessing int
	}
	mock.lockSetBlockProcessing.RLock()
	calls = mock.calls.SetBlockProcessing
	mock.lockSetBlockProcessing.RUnlock()
	return calls
}

// UpdateBlocksStatuses calls UpdateBlocksStatusesFunc.
func (mock *BlocktxStoreMock) UpdateBlocksStatuses(ctx context.Context, blockStatusUpdates []store.BlockStatusUpdate) error {
	if mock.UpdateBlocksStatusesFunc == nil {
		panic("BlocktxStoreMock.UpdateBlocksStatusesFunc: method is nil but BlocktxStore.UpdateBlocksStatuses was just called")
	}
	callInfo := struct {
		Ctx                context.Context
		BlockStatusUpdates []store.BlockStatusUpdate
	}{
		Ctx:                ctx,
		BlockStatusUpdates: blockStatusUpdates,
	}
	mock.lockUpdateBlocksStatuses.Lock()
	mock.calls.UpdateBlocksStatuses = append(mock.calls.UpdateBlocksStatuses, callInfo)
	mock.lockUpdateBlocksStatuses.Unlock()
	return mock.UpdateBlocksStatusesFunc(ctx, blockStatusUpdates)
}

// UpdateBlocksStatusesCalls gets all the calls that were made to UpdateBlocksStatuses.
// Check the length with:
//
//	len(mockedBlocktxStore.UpdateBlocksStatusesCalls())
func (mock *BlocktxStoreMock) UpdateBlocksStatusesCalls() []struct {
	Ctx                context.Context
	BlockStatusUpdates []store.BlockStatusUpdate
} {
	var calls []struct {
		Ctx                context.Context
		BlockStatusUpdates []store.BlockStatusUpdate
	}
	mock.lockUpdateBlocksStatuses.RLock()
	calls = mock.calls.UpdateBlocksStatuses
	mock.lockUpdateBlocksStatuses.RUnlock()
	return calls
}

// UpsertBlock calls UpsertBlockFunc.
func (mock *BlocktxStoreMock) UpsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	if mock.UpsertBlockFunc == nil {
		panic("BlocktxStoreMock.UpsertBlockFunc: method is nil but BlocktxStore.UpsertBlock was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Block *blocktx_api.Block
	}{
		Ctx:   ctx,
		Block: block,
	}
	mock.lockUpsertBlock.Lock()
	mock.calls.UpsertBlock = append(mock.calls.UpsertBlock, callInfo)
	mock.lockUpsertBlock.Unlock()
	return mock.UpsertBlockFunc(ctx, block)
}

// UpsertBlockCalls gets all the calls that were made to UpsertBlock.
// Check the length with:
//
//	len(mockedBlocktxStore.UpsertBlockCalls())
func (mock *BlocktxStoreMock) UpsertBlockCalls() []struct {
	Ctx   context.Context
	Block *blocktx_api.Block
} {
	var calls []struct {
		Ctx   context.Context
		Block *blocktx_api.Block
	}
	mock.lockUpsertBlock.RLock()
	calls = mock.calls.UpsertBlock
	mock.lockUpsertBlock.RUnlock()
	return calls
}

// VerifyMerkleRoots calls VerifyMerkleRootsFunc.
func (mock *BlocktxStoreMock) VerifyMerkleRoots(ctx context.Context, merkleRoots []*blocktx_api.MerkleRootVerificationRequest, maxAllowedBlockHeightMismatch int) (*blocktx_api.MerkleRootVerificationResponse, error) {
	if mock.VerifyMerkleRootsFunc == nil {
		panic("BlocktxStoreMock.VerifyMerkleRootsFunc: method is nil but BlocktxStore.VerifyMerkleRoots was just called")
	}
	callInfo := struct {
		Ctx                           context.Context
		MerkleRoots                   []*blocktx_api.MerkleRootVerificationRequest
		MaxAllowedBlockHeightMismatch int
	}{
		Ctx:                           ctx,
		MerkleRoots:                   merkleRoots,
		MaxAllowedBlockHeightMismatch: maxAllowedBlockHeightMismatch,
	}
	mock.lockVerifyMerkleRoots.Lock()
	mock.calls.VerifyMerkleRoots = append(mock.calls.VerifyMerkleRoots, callInfo)
	mock.lockVerifyMerkleRoots.Unlock()
	return mock.VerifyMerkleRootsFunc(ctx, merkleRoots, maxAllowedBlockHeightMismatch)
}

// VerifyMerkleRootsCalls gets all the calls that were made to VerifyMerkleRoots.
// Check the length with:
//
//	len(mockedBlocktxStore.VerifyMerkleRootsCalls())
func (mock *BlocktxStoreMock) VerifyMerkleRootsCalls() []struct {
	Ctx                           context.Context
	MerkleRoots                   []*blocktx_api.MerkleRootVerificationRequest
	MaxAllowedBlockHeightMismatch int
} {
	var calls []struct {
		Ctx                           context.Context
		MerkleRoots                   []*blocktx_api.MerkleRootVerificationRequest
		MaxAllowedBlockHeightMismatch int
	}
	mock.lockVerifyMerkleRoots.RLock()
	calls = mock.calls.VerifyMerkleRoots
	mock.lockVerifyMerkleRoots.RUnlock()
	return calls
}
