// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"sync"
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
//			DelBlockProcessingFunc: func(ctx context.Context, hash *chainhash.Hash, processedBy string) (int64, error) {
//				panic("mock out the DelBlockProcessing method")
//			},
//			GetBlockFunc: func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
//				panic("mock out the GetBlock method")
//			},
//			GetBlockByHeightFunc: func(ctx context.Context, height uint64, status blocktx_api.Status) (*blocktx_api.Block, error) {
//				panic("mock out the GetBlockByHeight method")
//			},
//			GetBlockGapsFunc: func(ctx context.Context, heightRange int) ([]*store.BlockGap, error) {
//				panic("mock out the GetBlockGaps method")
//			},
//			GetBlockHashesProcessingInProgressFunc: func(ctx context.Context, processedBy string) ([]*chainhash.Hash, error) {
//				panic("mock out the GetBlockHashesProcessingInProgress method")
//			},
//			GetChainTipFunc: func(ctx context.Context) (*blocktx_api.Block, error) {
//				panic("mock out the GetChainTip method")
//			},
//			GetMinedTransactionsFunc: func(ctx context.Context, hashes []*chainhash.Hash) ([]store.GetMinedTransactionResult, error) {
//				panic("mock out the GetMinedTransactions method")
//			},
//			InsertBlockFunc: func(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
//				panic("mock out the InsertBlock method")
//			},
//			MarkBlockAsDoneFunc: func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
//				panic("mock out the MarkBlockAsDone method")
//			},
//			PingFunc: func(ctx context.Context) error {
//				panic("mock out the Ping method")
//			},
//			RegisterTransactionsFunc: func(ctx context.Context, transaction []*blocktx_api.TransactionAndSource) ([]*chainhash.Hash, error) {
//				panic("mock out the RegisterTransactions method")
//			},
//			SetBlockProcessingFunc: func(ctx context.Context, hash *chainhash.Hash, processedBy string) (string, error) {
//				panic("mock out the SetBlockProcessing method")
//			},
//			UpsertBlockTransactionsFunc: func(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) ([]store.UpsertBlockTransactionsResult, error) {
//				panic("mock out the UpsertBlockTransactions method")
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

	// DelBlockProcessingFunc mocks the DelBlockProcessing method.
	DelBlockProcessingFunc func(ctx context.Context, hash *chainhash.Hash, processedBy string) (int64, error)

	// GetBlockFunc mocks the GetBlock method.
	GetBlockFunc func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error)

	// GetBlockByHeightFunc mocks the GetBlockByHeight method.
	GetBlockByHeightFunc func(ctx context.Context, height uint64, status blocktx_api.Status) (*blocktx_api.Block, error)

	// GetBlockGapsFunc mocks the GetBlockGaps method.
	GetBlockGapsFunc func(ctx context.Context, heightRange int) ([]*store.BlockGap, error)

	// GetBlockHashesProcessingInProgressFunc mocks the GetBlockHashesProcessingInProgress method.
	GetBlockHashesProcessingInProgressFunc func(ctx context.Context, processedBy string) ([]*chainhash.Hash, error)

	// GetChainTipFunc mocks the GetChainTip method.
	GetChainTipFunc func(ctx context.Context) (*blocktx_api.Block, error)

	// GetMinedTransactionsFunc mocks the GetMinedTransactions method.
	GetMinedTransactionsFunc func(ctx context.Context, hashes []*chainhash.Hash) ([]store.GetMinedTransactionResult, error)

	// InsertBlockFunc mocks the InsertBlock method.
	InsertBlockFunc func(ctx context.Context, block *blocktx_api.Block) (uint64, error)

	// MarkBlockAsDoneFunc mocks the MarkBlockAsDone method.
	MarkBlockAsDoneFunc func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error

	// PingFunc mocks the Ping method.
	PingFunc func(ctx context.Context) error

	// RegisterTransactionsFunc mocks the RegisterTransactions method.
	RegisterTransactionsFunc func(ctx context.Context, transaction []*blocktx_api.TransactionAndSource) ([]*chainhash.Hash, error)

	// SetBlockProcessingFunc mocks the SetBlockProcessing method.
	SetBlockProcessingFunc func(ctx context.Context, hash *chainhash.Hash, processedBy string) (string, error)

	// UpsertBlockTransactionsFunc mocks the UpsertBlockTransactions method.
	UpsertBlockTransactionsFunc func(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) ([]store.UpsertBlockTransactionsResult, error)

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
		// DelBlockProcessing holds details about calls to the DelBlockProcessing method.
		DelBlockProcessing []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
			// ProcessedBy is the processedBy argument value.
			ProcessedBy string
		}
		// GetBlock holds details about calls to the GetBlock method.
		GetBlock []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
		}
		// GetBlockByHeight holds details about calls to the GetBlockByHeight method.
		GetBlockByHeight []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Height is the height argument value.
			Height uint64
			// Status is the status argument value.
			Status blocktx_api.Status
		}
		// GetBlockGaps holds details about calls to the GetBlockGaps method.
		GetBlockGaps []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// HeightRange is the heightRange argument value.
			HeightRange int
		}
		// GetBlockHashesProcessingInProgress holds details about calls to the GetBlockHashesProcessingInProgress method.
		GetBlockHashesProcessingInProgress []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// ProcessedBy is the processedBy argument value.
			ProcessedBy string
		}
		// GetChainTip holds details about calls to the GetChainTip method.
		GetChainTip []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// GetMinedTransactions holds details about calls to the GetMinedTransactions method.
		GetMinedTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hashes is the hashes argument value.
			Hashes []*chainhash.Hash
		}
		// InsertBlock holds details about calls to the InsertBlock method.
		InsertBlock []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Block is the block argument value.
			Block *blocktx_api.Block
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
			// Transaction is the transaction argument value.
			Transaction []*blocktx_api.TransactionAndSource
		}
		// SetBlockProcessing holds details about calls to the SetBlockProcessing method.
		SetBlockProcessing []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
			// ProcessedBy is the processedBy argument value.
			ProcessedBy string
		}
		// UpsertBlockTransactions holds details about calls to the UpsertBlockTransactions method.
		UpsertBlockTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockId is the blockId argument value.
			BlockId uint64
			// Transactions is the transactions argument value.
			Transactions []*blocktx_api.TransactionAndSource
			// MerklePaths is the merklePaths argument value.
			MerklePaths []string
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
	lockClearBlocktxTable                  sync.RWMutex
	lockClose                              sync.RWMutex
	lockDelBlockProcessing                 sync.RWMutex
	lockGetBlock                           sync.RWMutex
	lockGetBlockByHeight                   sync.RWMutex
	lockGetBlockGaps                       sync.RWMutex
	lockGetBlockHashesProcessingInProgress sync.RWMutex
	lockGetChainTip                        sync.RWMutex
	lockGetMinedTransactions               sync.RWMutex
	lockInsertBlock                        sync.RWMutex
	lockMarkBlockAsDone                    sync.RWMutex
	lockPing                               sync.RWMutex
	lockRegisterTransactions               sync.RWMutex
	lockSetBlockProcessing                 sync.RWMutex
	lockUpsertBlockTransactions            sync.RWMutex
	lockVerifyMerkleRoots                  sync.RWMutex
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

// DelBlockProcessing calls DelBlockProcessingFunc.
func (mock *BlocktxStoreMock) DelBlockProcessing(ctx context.Context, hash *chainhash.Hash, processedBy string) (int64, error) {
	if mock.DelBlockProcessingFunc == nil {
		panic("BlocktxStoreMock.DelBlockProcessingFunc: method is nil but BlocktxStore.DelBlockProcessing was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		Hash        *chainhash.Hash
		ProcessedBy string
	}{
		Ctx:         ctx,
		Hash:        hash,
		ProcessedBy: processedBy,
	}
	mock.lockDelBlockProcessing.Lock()
	mock.calls.DelBlockProcessing = append(mock.calls.DelBlockProcessing, callInfo)
	mock.lockDelBlockProcessing.Unlock()
	return mock.DelBlockProcessingFunc(ctx, hash, processedBy)
}

// DelBlockProcessingCalls gets all the calls that were made to DelBlockProcessing.
// Check the length with:
//
//	len(mockedBlocktxStore.DelBlockProcessingCalls())
func (mock *BlocktxStoreMock) DelBlockProcessingCalls() []struct {
	Ctx         context.Context
	Hash        *chainhash.Hash
	ProcessedBy string
} {
	var calls []struct {
		Ctx         context.Context
		Hash        *chainhash.Hash
		ProcessedBy string
	}
	mock.lockDelBlockProcessing.RLock()
	calls = mock.calls.DelBlockProcessing
	mock.lockDelBlockProcessing.RUnlock()
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

// GetBlockByHeight calls GetBlockByHeightFunc.
func (mock *BlocktxStoreMock) GetBlockByHeight(ctx context.Context, height uint64, status blocktx_api.Status) (*blocktx_api.Block, error) {
	if mock.GetBlockByHeightFunc == nil {
		panic("BlocktxStoreMock.GetBlockByHeightFunc: method is nil but BlocktxStore.GetBlockByHeight was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Height uint64
		Status blocktx_api.Status
	}{
		Ctx:    ctx,
		Height: height,
		Status: status,
	}
	mock.lockGetBlockByHeight.Lock()
	mock.calls.GetBlockByHeight = append(mock.calls.GetBlockByHeight, callInfo)
	mock.lockGetBlockByHeight.Unlock()
	return mock.GetBlockByHeightFunc(ctx, height, status)
}

// GetBlockByHeightCalls gets all the calls that were made to GetBlockByHeight.
// Check the length with:
//
//	len(mockedBlocktxStore.GetBlockByHeightCalls())
func (mock *BlocktxStoreMock) GetBlockByHeightCalls() []struct {
	Ctx    context.Context
	Height uint64
	Status blocktx_api.Status
} {
	var calls []struct {
		Ctx    context.Context
		Height uint64
		Status blocktx_api.Status
	}
	mock.lockGetBlockByHeight.RLock()
	calls = mock.calls.GetBlockByHeight
	mock.lockGetBlockByHeight.RUnlock()
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

// GetBlockHashesProcessingInProgress calls GetBlockHashesProcessingInProgressFunc.
func (mock *BlocktxStoreMock) GetBlockHashesProcessingInProgress(ctx context.Context, processedBy string) ([]*chainhash.Hash, error) {
	if mock.GetBlockHashesProcessingInProgressFunc == nil {
		panic("BlocktxStoreMock.GetBlockHashesProcessingInProgressFunc: method is nil but BlocktxStore.GetBlockHashesProcessingInProgress was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		ProcessedBy string
	}{
		Ctx:         ctx,
		ProcessedBy: processedBy,
	}
	mock.lockGetBlockHashesProcessingInProgress.Lock()
	mock.calls.GetBlockHashesProcessingInProgress = append(mock.calls.GetBlockHashesProcessingInProgress, callInfo)
	mock.lockGetBlockHashesProcessingInProgress.Unlock()
	return mock.GetBlockHashesProcessingInProgressFunc(ctx, processedBy)
}

// GetBlockHashesProcessingInProgressCalls gets all the calls that were made to GetBlockHashesProcessingInProgress.
// Check the length with:
//
//	len(mockedBlocktxStore.GetBlockHashesProcessingInProgressCalls())
func (mock *BlocktxStoreMock) GetBlockHashesProcessingInProgressCalls() []struct {
	Ctx         context.Context
	ProcessedBy string
} {
	var calls []struct {
		Ctx         context.Context
		ProcessedBy string
	}
	mock.lockGetBlockHashesProcessingInProgress.RLock()
	calls = mock.calls.GetBlockHashesProcessingInProgress
	mock.lockGetBlockHashesProcessingInProgress.RUnlock()
	return calls
}

// GetChainTip calls GetChainTipFunc.
func (mock *BlocktxStoreMock) GetChainTip(ctx context.Context) (*blocktx_api.Block, error) {
	if mock.GetChainTipFunc == nil {
		panic("BlocktxStoreMock.GetChainTipFunc: method is nil but BlocktxStore.GetChainTip was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetChainTip.Lock()
	mock.calls.GetChainTip = append(mock.calls.GetChainTip, callInfo)
	mock.lockGetChainTip.Unlock()
	return mock.GetChainTipFunc(ctx)
}

// GetChainTipCalls gets all the calls that were made to GetChainTip.
// Check the length with:
//
//	len(mockedBlocktxStore.GetChainTipCalls())
func (mock *BlocktxStoreMock) GetChainTipCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetChainTip.RLock()
	calls = mock.calls.GetChainTip
	mock.lockGetChainTip.RUnlock()
	return calls
}

// GetMinedTransactions calls GetMinedTransactionsFunc.
func (mock *BlocktxStoreMock) GetMinedTransactions(ctx context.Context, hashes []*chainhash.Hash) ([]store.GetMinedTransactionResult, error) {
	if mock.GetMinedTransactionsFunc == nil {
		panic("BlocktxStoreMock.GetMinedTransactionsFunc: method is nil but BlocktxStore.GetMinedTransactions was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Hashes []*chainhash.Hash
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
	Hashes []*chainhash.Hash
} {
	var calls []struct {
		Ctx    context.Context
		Hashes []*chainhash.Hash
	}
	mock.lockGetMinedTransactions.RLock()
	calls = mock.calls.GetMinedTransactions
	mock.lockGetMinedTransactions.RUnlock()
	return calls
}

// InsertBlock calls InsertBlockFunc.
func (mock *BlocktxStoreMock) InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	if mock.InsertBlockFunc == nil {
		panic("BlocktxStoreMock.InsertBlockFunc: method is nil but BlocktxStore.InsertBlock was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Block *blocktx_api.Block
	}{
		Ctx:   ctx,
		Block: block,
	}
	mock.lockInsertBlock.Lock()
	mock.calls.InsertBlock = append(mock.calls.InsertBlock, callInfo)
	mock.lockInsertBlock.Unlock()
	return mock.InsertBlockFunc(ctx, block)
}

// InsertBlockCalls gets all the calls that were made to InsertBlock.
// Check the length with:
//
//	len(mockedBlocktxStore.InsertBlockCalls())
func (mock *BlocktxStoreMock) InsertBlockCalls() []struct {
	Ctx   context.Context
	Block *blocktx_api.Block
} {
	var calls []struct {
		Ctx   context.Context
		Block *blocktx_api.Block
	}
	mock.lockInsertBlock.RLock()
	calls = mock.calls.InsertBlock
	mock.lockInsertBlock.RUnlock()
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
func (mock *BlocktxStoreMock) RegisterTransactions(ctx context.Context, transaction []*blocktx_api.TransactionAndSource) ([]*chainhash.Hash, error) {
	if mock.RegisterTransactionsFunc == nil {
		panic("BlocktxStoreMock.RegisterTransactionsFunc: method is nil but BlocktxStore.RegisterTransactions was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		Transaction []*blocktx_api.TransactionAndSource
	}{
		Ctx:         ctx,
		Transaction: transaction,
	}
	mock.lockRegisterTransactions.Lock()
	mock.calls.RegisterTransactions = append(mock.calls.RegisterTransactions, callInfo)
	mock.lockRegisterTransactions.Unlock()
	return mock.RegisterTransactionsFunc(ctx, transaction)
}

// RegisterTransactionsCalls gets all the calls that were made to RegisterTransactions.
// Check the length with:
//
//	len(mockedBlocktxStore.RegisterTransactionsCalls())
func (mock *BlocktxStoreMock) RegisterTransactionsCalls() []struct {
	Ctx         context.Context
	Transaction []*blocktx_api.TransactionAndSource
} {
	var calls []struct {
		Ctx         context.Context
		Transaction []*blocktx_api.TransactionAndSource
	}
	mock.lockRegisterTransactions.RLock()
	calls = mock.calls.RegisterTransactions
	mock.lockRegisterTransactions.RUnlock()
	return calls
}

// SetBlockProcessing calls SetBlockProcessingFunc.
func (mock *BlocktxStoreMock) SetBlockProcessing(ctx context.Context, hash *chainhash.Hash, processedBy string) (string, error) {
	if mock.SetBlockProcessingFunc == nil {
		panic("BlocktxStoreMock.SetBlockProcessingFunc: method is nil but BlocktxStore.SetBlockProcessing was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		Hash        *chainhash.Hash
		ProcessedBy string
	}{
		Ctx:         ctx,
		Hash:        hash,
		ProcessedBy: processedBy,
	}
	mock.lockSetBlockProcessing.Lock()
	mock.calls.SetBlockProcessing = append(mock.calls.SetBlockProcessing, callInfo)
	mock.lockSetBlockProcessing.Unlock()
	return mock.SetBlockProcessingFunc(ctx, hash, processedBy)
}

// SetBlockProcessingCalls gets all the calls that were made to SetBlockProcessing.
// Check the length with:
//
//	len(mockedBlocktxStore.SetBlockProcessingCalls())
func (mock *BlocktxStoreMock) SetBlockProcessingCalls() []struct {
	Ctx         context.Context
	Hash        *chainhash.Hash
	ProcessedBy string
} {
	var calls []struct {
		Ctx         context.Context
		Hash        *chainhash.Hash
		ProcessedBy string
	}
	mock.lockSetBlockProcessing.RLock()
	calls = mock.calls.SetBlockProcessing
	mock.lockSetBlockProcessing.RUnlock()
	return calls
}

// UpsertBlockTransactions calls UpsertBlockTransactionsFunc.
func (mock *BlocktxStoreMock) UpsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) ([]store.UpsertBlockTransactionsResult, error) {
	if mock.UpsertBlockTransactionsFunc == nil {
		panic("BlocktxStoreMock.UpsertBlockTransactionsFunc: method is nil but BlocktxStore.UpsertBlockTransactions was just called")
	}
	callInfo := struct {
		Ctx          context.Context
		BlockId      uint64
		Transactions []*blocktx_api.TransactionAndSource
		MerklePaths  []string
	}{
		Ctx:          ctx,
		BlockId:      blockId,
		Transactions: transactions,
		MerklePaths:  merklePaths,
	}
	mock.lockUpsertBlockTransactions.Lock()
	mock.calls.UpsertBlockTransactions = append(mock.calls.UpsertBlockTransactions, callInfo)
	mock.lockUpsertBlockTransactions.Unlock()
	return mock.UpsertBlockTransactionsFunc(ctx, blockId, transactions, merklePaths)
}

// UpsertBlockTransactionsCalls gets all the calls that were made to UpsertBlockTransactions.
// Check the length with:
//
//	len(mockedBlocktxStore.UpsertBlockTransactionsCalls())
func (mock *BlocktxStoreMock) UpsertBlockTransactionsCalls() []struct {
	Ctx          context.Context
	BlockId      uint64
	Transactions []*blocktx_api.TransactionAndSource
	MerklePaths  []string
} {
	var calls []struct {
		Ctx          context.Context
		BlockId      uint64
		Transactions []*blocktx_api.TransactionAndSource
		MerklePaths  []string
	}
	mock.lockUpsertBlockTransactions.RLock()
	calls = mock.calls.UpsertBlockTransactions
	mock.lockUpsertBlockTransactions.RUnlock()
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
