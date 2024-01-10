// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package store

import (
	"context"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"sync"
)

// Ensure, that InterfaceMock does implement Interface.
// If this is not the case, regenerate this file with moq.
var _ Interface = &InterfaceMock{}

// InterfaceMock is a mock implementation of Interface.
//
//	func TestSomethingThatUsesInterface(t *testing.T) {
//
//		// make and configure a mocked Interface
//		mockedInterface := &InterfaceMock{
//			CloseFunc: func() error {
//				panic("mock out the Close method")
//			},
//			GetBlockFunc: func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
//				panic("mock out the GetBlock method")
//			},
//			GetBlockForHeightFunc: func(ctx context.Context, height uint64) (*blocktx_api.Block, error) {
//				panic("mock out the GetBlockForHeight method")
//			},
//			GetBlockGapsFunc: func(ctx context.Context) ([]*BlockGap, error) {
//				panic("mock out the GetBlockGaps method")
//			},
//			GetBlockTransactionsFunc: func(ctx context.Context, block *blocktx_api.Block) (*blocktx_api.Transactions, error) {
//				panic("mock out the GetBlockTransactions method")
//			},
//			GetLastProcessedBlockFunc: func(ctx context.Context) (*blocktx_api.Block, error) {
//				panic("mock out the GetLastProcessedBlock method")
//			},
//			GetMinedTransactionsForBlockFunc: func(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
//				panic("mock out the GetMinedTransactionsForBlock method")
//			},
//			GetTransactionBlockFunc: func(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error) {
//				panic("mock out the GetTransactionBlock method")
//			},
//			GetTransactionBlocksFunc: func(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
//				panic("mock out the GetTransactionBlocks method")
//			},
//			GetTransactionMerklePathFunc: func(ctx context.Context, hash *chainhash.Hash) (string, error) {
//				panic("mock out the GetTransactionMerklePath method")
//			},
//			InsertBlockFunc: func(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
//				panic("mock out the InsertBlock method")
//			},
//			InsertBlockTransactionsFunc: func(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) error {
//				panic("mock out the InsertBlockTransactions method")
//			},
//			MarkBlockAsDoneFunc: func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
//				panic("mock out the MarkBlockAsDone method")
//			},
//			OrphanHeightFunc: func(ctx context.Context, height uint64) error {
//				panic("mock out the OrphanHeight method")
//			},
//			PrimaryBlocktxFunc: func(ctx context.Context) (string, error) {
//				panic("mock out the PrimaryBlocktx method")
//			},
//			RegisterTransactionFunc: func(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (string, string, []byte, uint64, error) {
//				panic("mock out the RegisterTransaction method")
//			},
//			SetOrphanHeightFunc: func(ctx context.Context, height uint64, orphaned bool) error {
//				panic("mock out the SetOrphanHeight method")
//			},
//			TryToBecomePrimaryFunc: func(ctx context.Context, myHostName string) error {
//				panic("mock out the TryToBecomePrimary method")
//			},
//		}
//
//		// use mockedInterface in code that requires Interface
//		// and then make assertions.
//
//	}
type InterfaceMock struct {
	// CloseFunc mocks the Close method.
	CloseFunc func() error

	// GetBlockFunc mocks the GetBlock method.
	GetBlockFunc func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error)

	// GetBlockForHeightFunc mocks the GetBlockForHeight method.
	GetBlockForHeightFunc func(ctx context.Context, height uint64) (*blocktx_api.Block, error)

	// GetBlockGapsFunc mocks the GetBlockGaps method.
	GetBlockGapsFunc func(ctx context.Context) ([]*BlockGap, error)

	// GetBlockTransactionsFunc mocks the GetBlockTransactions method.
	GetBlockTransactionsFunc func(ctx context.Context, block *blocktx_api.Block) (*blocktx_api.Transactions, error)

	// GetLastProcessedBlockFunc mocks the GetLastProcessedBlock method.
	GetLastProcessedBlockFunc func(ctx context.Context) (*blocktx_api.Block, error)

	// GetMinedTransactionsForBlockFunc mocks the GetMinedTransactionsForBlock method.
	GetMinedTransactionsForBlockFunc func(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error)

	// GetTransactionBlockFunc mocks the GetTransactionBlock method.
	GetTransactionBlockFunc func(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error)

	// GetTransactionBlocksFunc mocks the GetTransactionBlocks method.
	GetTransactionBlocksFunc func(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error)

	// GetTransactionMerklePathFunc mocks the GetTransactionMerklePath method.
	GetTransactionMerklePathFunc func(ctx context.Context, hash *chainhash.Hash) (string, error)

	// InsertBlockFunc mocks the InsertBlock method.
	InsertBlockFunc func(ctx context.Context, block *blocktx_api.Block) (uint64, error)

	// InsertBlockTransactionsFunc mocks the InsertBlockTransactions method.
	InsertBlockTransactionsFunc func(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) error

	// MarkBlockAsDoneFunc mocks the MarkBlockAsDone method.
	MarkBlockAsDoneFunc func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error

	// OrphanHeightFunc mocks the OrphanHeight method.
	OrphanHeightFunc func(ctx context.Context, height uint64) error

	// PrimaryBlocktxFunc mocks the PrimaryBlocktx method.
	PrimaryBlocktxFunc func(ctx context.Context) (string, error)

	// RegisterTransactionFunc mocks the RegisterTransaction method.
	RegisterTransactionFunc func(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (string, string, []byte, uint64, error)

	// SetOrphanHeightFunc mocks the SetOrphanHeight method.
	SetOrphanHeightFunc func(ctx context.Context, height uint64, orphaned bool) error

	// TryToBecomePrimaryFunc mocks the TryToBecomePrimary method.
	TryToBecomePrimaryFunc func(ctx context.Context, myHostName string) error

	// calls tracks calls to the methods.
	calls struct {
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
		// GetBlockForHeight holds details about calls to the GetBlockForHeight method.
		GetBlockForHeight []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Height is the height argument value.
			Height uint64
		}
		// GetBlockGaps holds details about calls to the GetBlockGaps method.
		GetBlockGaps []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// GetBlockTransactions holds details about calls to the GetBlockTransactions method.
		GetBlockTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Block is the block argument value.
			Block *blocktx_api.Block
		}
		// GetLastProcessedBlock holds details about calls to the GetLastProcessedBlock method.
		GetLastProcessedBlock []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// GetMinedTransactionsForBlock holds details about calls to the GetMinedTransactionsForBlock method.
		GetMinedTransactionsForBlock []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockAndSource is the blockAndSource argument value.
			BlockAndSource *blocktx_api.BlockAndSource
		}
		// GetTransactionBlock holds details about calls to the GetTransactionBlock method.
		GetTransactionBlock []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Transaction is the transaction argument value.
			Transaction *blocktx_api.Transaction
		}
		// GetTransactionBlocks holds details about calls to the GetTransactionBlocks method.
		GetTransactionBlocks []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Transactions is the transactions argument value.
			Transactions *blocktx_api.Transactions
		}
		// GetTransactionMerklePath holds details about calls to the GetTransactionMerklePath method.
		GetTransactionMerklePath []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
		}
		// InsertBlock holds details about calls to the InsertBlock method.
		InsertBlock []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Block is the block argument value.
			Block *blocktx_api.Block
		}
		// InsertBlockTransactions holds details about calls to the InsertBlockTransactions method.
		InsertBlockTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockId is the blockId argument value.
			BlockId uint64
			// Transactions is the transactions argument value.
			Transactions []*blocktx_api.TransactionAndSource
			// MerklePaths is the merklePaths argument value.
			MerklePaths []string
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
		// OrphanHeight holds details about calls to the OrphanHeight method.
		OrphanHeight []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Height is the height argument value.
			Height uint64
		}
		// PrimaryBlocktx holds details about calls to the PrimaryBlocktx method.
		PrimaryBlocktx []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// RegisterTransaction holds details about calls to the RegisterTransaction method.
		RegisterTransaction []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Transaction is the transaction argument value.
			Transaction *blocktx_api.TransactionAndSource
		}
		// SetOrphanHeight holds details about calls to the SetOrphanHeight method.
		SetOrphanHeight []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Height is the height argument value.
			Height uint64
			// Orphaned is the orphaned argument value.
			Orphaned bool
		}
		// TryToBecomePrimary holds details about calls to the TryToBecomePrimary method.
		TryToBecomePrimary []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// MyHostName is the myHostName argument value.
			MyHostName string
		}
	}
	lockClose                        sync.RWMutex
	lockGetBlock                     sync.RWMutex
	lockGetBlockForHeight            sync.RWMutex
	lockGetBlockGaps                 sync.RWMutex
	lockGetBlockTransactions         sync.RWMutex
	lockGetLastProcessedBlock        sync.RWMutex
	lockGetMinedTransactionsForBlock sync.RWMutex
	lockGetTransactionBlock          sync.RWMutex
	lockGetTransactionBlocks         sync.RWMutex
	lockGetTransactionMerklePath     sync.RWMutex
	lockInsertBlock                  sync.RWMutex
	lockInsertBlockTransactions      sync.RWMutex
	lockMarkBlockAsDone              sync.RWMutex
	lockOrphanHeight                 sync.RWMutex
	lockPrimaryBlocktx               sync.RWMutex
	lockRegisterTransaction          sync.RWMutex
	lockSetOrphanHeight              sync.RWMutex
	lockTryToBecomePrimary           sync.RWMutex
}

// Close calls CloseFunc.
func (mock *InterfaceMock) Close() error {
	if mock.CloseFunc == nil {
		panic("InterfaceMock.CloseFunc: method is nil but Interface.Close was just called")
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
//	len(mockedInterface.CloseCalls())
func (mock *InterfaceMock) CloseCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockClose.RLock()
	calls = mock.calls.Close
	mock.lockClose.RUnlock()
	return calls
}

// GetBlock calls GetBlockFunc.
func (mock *InterfaceMock) GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
	if mock.GetBlockFunc == nil {
		panic("InterfaceMock.GetBlockFunc: method is nil but Interface.GetBlock was just called")
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
//	len(mockedInterface.GetBlockCalls())
func (mock *InterfaceMock) GetBlockCalls() []struct {
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

// GetBlockForHeight calls GetBlockForHeightFunc.
func (mock *InterfaceMock) GetBlockForHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error) {
	if mock.GetBlockForHeightFunc == nil {
		panic("InterfaceMock.GetBlockForHeightFunc: method is nil but Interface.GetBlockForHeight was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Height uint64
	}{
		Ctx:    ctx,
		Height: height,
	}
	mock.lockGetBlockForHeight.Lock()
	mock.calls.GetBlockForHeight = append(mock.calls.GetBlockForHeight, callInfo)
	mock.lockGetBlockForHeight.Unlock()
	return mock.GetBlockForHeightFunc(ctx, height)
}

// GetBlockForHeightCalls gets all the calls that were made to GetBlockForHeight.
// Check the length with:
//
//	len(mockedInterface.GetBlockForHeightCalls())
func (mock *InterfaceMock) GetBlockForHeightCalls() []struct {
	Ctx    context.Context
	Height uint64
} {
	var calls []struct {
		Ctx    context.Context
		Height uint64
	}
	mock.lockGetBlockForHeight.RLock()
	calls = mock.calls.GetBlockForHeight
	mock.lockGetBlockForHeight.RUnlock()
	return calls
}

// GetBlockGaps calls GetBlockGapsFunc.
func (mock *InterfaceMock) GetBlockGaps(ctx context.Context, heightRange int) ([]*BlockGap, error) {
	if mock.GetBlockGapsFunc == nil {
		panic("InterfaceMock.GetBlockGapsFunc: method is nil but Interface.GetBlockGaps was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetBlockGaps.Lock()
	mock.calls.GetBlockGaps = append(mock.calls.GetBlockGaps, callInfo)
	mock.lockGetBlockGaps.Unlock()
	return mock.GetBlockGapsFunc(ctx)
}

// GetBlockGapsCalls gets all the calls that were made to GetBlockGaps.
// Check the length with:
//
//	len(mockedInterface.GetBlockGapsCalls())
func (mock *InterfaceMock) GetBlockGapsCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetBlockGaps.RLock()
	calls = mock.calls.GetBlockGaps
	mock.lockGetBlockGaps.RUnlock()
	return calls
}

// GetBlockTransactions calls GetBlockTransactionsFunc.
func (mock *InterfaceMock) GetBlockTransactions(ctx context.Context, block *blocktx_api.Block) (*blocktx_api.Transactions, error) {
	if mock.GetBlockTransactionsFunc == nil {
		panic("InterfaceMock.GetBlockTransactionsFunc: method is nil but Interface.GetBlockTransactions was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Block *blocktx_api.Block
	}{
		Ctx:   ctx,
		Block: block,
	}
	mock.lockGetBlockTransactions.Lock()
	mock.calls.GetBlockTransactions = append(mock.calls.GetBlockTransactions, callInfo)
	mock.lockGetBlockTransactions.Unlock()
	return mock.GetBlockTransactionsFunc(ctx, block)
}

// GetBlockTransactionsCalls gets all the calls that were made to GetBlockTransactions.
// Check the length with:
//
//	len(mockedInterface.GetBlockTransactionsCalls())
func (mock *InterfaceMock) GetBlockTransactionsCalls() []struct {
	Ctx   context.Context
	Block *blocktx_api.Block
} {
	var calls []struct {
		Ctx   context.Context
		Block *blocktx_api.Block
	}
	mock.lockGetBlockTransactions.RLock()
	calls = mock.calls.GetBlockTransactions
	mock.lockGetBlockTransactions.RUnlock()
	return calls
}

// GetLastProcessedBlock calls GetLastProcessedBlockFunc.
func (mock *InterfaceMock) GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error) {
	if mock.GetLastProcessedBlockFunc == nil {
		panic("InterfaceMock.GetLastProcessedBlockFunc: method is nil but Interface.GetLastProcessedBlock was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetLastProcessedBlock.Lock()
	mock.calls.GetLastProcessedBlock = append(mock.calls.GetLastProcessedBlock, callInfo)
	mock.lockGetLastProcessedBlock.Unlock()
	return mock.GetLastProcessedBlockFunc(ctx)
}

// GetLastProcessedBlockCalls gets all the calls that were made to GetLastProcessedBlock.
// Check the length with:
//
//	len(mockedInterface.GetLastProcessedBlockCalls())
func (mock *InterfaceMock) GetLastProcessedBlockCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetLastProcessedBlock.RLock()
	calls = mock.calls.GetLastProcessedBlock
	mock.lockGetLastProcessedBlock.RUnlock()
	return calls
}

// GetMinedTransactionsForBlock calls GetMinedTransactionsForBlockFunc.
func (mock *InterfaceMock) GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
	if mock.GetMinedTransactionsForBlockFunc == nil {
		panic("InterfaceMock.GetMinedTransactionsForBlockFunc: method is nil but Interface.GetMinedTransactionsForBlock was just called")
	}
	callInfo := struct {
		Ctx            context.Context
		BlockAndSource *blocktx_api.BlockAndSource
	}{
		Ctx:            ctx,
		BlockAndSource: blockAndSource,
	}
	mock.lockGetMinedTransactionsForBlock.Lock()
	mock.calls.GetMinedTransactionsForBlock = append(mock.calls.GetMinedTransactionsForBlock, callInfo)
	mock.lockGetMinedTransactionsForBlock.Unlock()
	return mock.GetMinedTransactionsForBlockFunc(ctx, blockAndSource)
}

// GetMinedTransactionsForBlockCalls gets all the calls that were made to GetMinedTransactionsForBlock.
// Check the length with:
//
//	len(mockedInterface.GetMinedTransactionsForBlockCalls())
func (mock *InterfaceMock) GetMinedTransactionsForBlockCalls() []struct {
	Ctx            context.Context
	BlockAndSource *blocktx_api.BlockAndSource
} {
	var calls []struct {
		Ctx            context.Context
		BlockAndSource *blocktx_api.BlockAndSource
	}
	mock.lockGetMinedTransactionsForBlock.RLock()
	calls = mock.calls.GetMinedTransactionsForBlock
	mock.lockGetMinedTransactionsForBlock.RUnlock()
	return calls
}

// GetTransactionBlock calls GetTransactionBlockFunc.
func (mock *InterfaceMock) GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error) {
	if mock.GetTransactionBlockFunc == nil {
		panic("InterfaceMock.GetTransactionBlockFunc: method is nil but Interface.GetTransactionBlock was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		Transaction *blocktx_api.Transaction
	}{
		Ctx:         ctx,
		Transaction: transaction,
	}
	mock.lockGetTransactionBlock.Lock()
	mock.calls.GetTransactionBlock = append(mock.calls.GetTransactionBlock, callInfo)
	mock.lockGetTransactionBlock.Unlock()
	return mock.GetTransactionBlockFunc(ctx, transaction)
}

// GetTransactionBlockCalls gets all the calls that were made to GetTransactionBlock.
// Check the length with:
//
//	len(mockedInterface.GetTransactionBlockCalls())
func (mock *InterfaceMock) GetTransactionBlockCalls() []struct {
	Ctx         context.Context
	Transaction *blocktx_api.Transaction
} {
	var calls []struct {
		Ctx         context.Context
		Transaction *blocktx_api.Transaction
	}
	mock.lockGetTransactionBlock.RLock()
	calls = mock.calls.GetTransactionBlock
	mock.lockGetTransactionBlock.RUnlock()
	return calls
}

// GetTransactionBlocks calls GetTransactionBlocksFunc.
func (mock *InterfaceMock) GetTransactionBlocks(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	if mock.GetTransactionBlocksFunc == nil {
		panic("InterfaceMock.GetTransactionBlocksFunc: method is nil but Interface.GetTransactionBlocks was just called")
	}
	callInfo := struct {
		Ctx          context.Context
		Transactions *blocktx_api.Transactions
	}{
		Ctx:          ctx,
		Transactions: transactions,
	}
	mock.lockGetTransactionBlocks.Lock()
	mock.calls.GetTransactionBlocks = append(mock.calls.GetTransactionBlocks, callInfo)
	mock.lockGetTransactionBlocks.Unlock()
	return mock.GetTransactionBlocksFunc(ctx, transactions)
}

// GetTransactionBlocksCalls gets all the calls that were made to GetTransactionBlocks.
// Check the length with:
//
//	len(mockedInterface.GetTransactionBlocksCalls())
func (mock *InterfaceMock) GetTransactionBlocksCalls() []struct {
	Ctx          context.Context
	Transactions *blocktx_api.Transactions
} {
	var calls []struct {
		Ctx          context.Context
		Transactions *blocktx_api.Transactions
	}
	mock.lockGetTransactionBlocks.RLock()
	calls = mock.calls.GetTransactionBlocks
	mock.lockGetTransactionBlocks.RUnlock()
	return calls
}

// GetTransactionMerklePath calls GetTransactionMerklePathFunc.
func (mock *InterfaceMock) GetTransactionMerklePath(ctx context.Context, hash *chainhash.Hash) (string, error) {
	if mock.GetTransactionMerklePathFunc == nil {
		panic("InterfaceMock.GetTransactionMerklePathFunc: method is nil but Interface.GetTransactionMerklePath was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Hash *chainhash.Hash
	}{
		Ctx:  ctx,
		Hash: hash,
	}
	mock.lockGetTransactionMerklePath.Lock()
	mock.calls.GetTransactionMerklePath = append(mock.calls.GetTransactionMerklePath, callInfo)
	mock.lockGetTransactionMerklePath.Unlock()
	return mock.GetTransactionMerklePathFunc(ctx, hash)
}

// GetTransactionMerklePathCalls gets all the calls that were made to GetTransactionMerklePath.
// Check the length with:
//
//	len(mockedInterface.GetTransactionMerklePathCalls())
func (mock *InterfaceMock) GetTransactionMerklePathCalls() []struct {
	Ctx  context.Context
	Hash *chainhash.Hash
} {
	var calls []struct {
		Ctx  context.Context
		Hash *chainhash.Hash
	}
	mock.lockGetTransactionMerklePath.RLock()
	calls = mock.calls.GetTransactionMerklePath
	mock.lockGetTransactionMerklePath.RUnlock()
	return calls
}

// InsertBlock calls InsertBlockFunc.
func (mock *InterfaceMock) InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	if mock.InsertBlockFunc == nil {
		panic("InterfaceMock.InsertBlockFunc: method is nil but Interface.InsertBlock was just called")
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
//	len(mockedInterface.InsertBlockCalls())
func (mock *InterfaceMock) InsertBlockCalls() []struct {
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

// InsertBlockTransactions calls InsertBlockTransactionsFunc.
func (mock *InterfaceMock) InsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) error {
	if mock.InsertBlockTransactionsFunc == nil {
		panic("InterfaceMock.InsertBlockTransactionsFunc: method is nil but Interface.InsertBlockTransactions was just called")
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
	mock.lockInsertBlockTransactions.Lock()
	mock.calls.InsertBlockTransactions = append(mock.calls.InsertBlockTransactions, callInfo)
	mock.lockInsertBlockTransactions.Unlock()
	return mock.InsertBlockTransactionsFunc(ctx, blockId, transactions, merklePaths)
}

// InsertBlockTransactionsCalls gets all the calls that were made to InsertBlockTransactions.
// Check the length with:
//
//	len(mockedInterface.InsertBlockTransactionsCalls())
func (mock *InterfaceMock) InsertBlockTransactionsCalls() []struct {
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
	mock.lockInsertBlockTransactions.RLock()
	calls = mock.calls.InsertBlockTransactions
	mock.lockInsertBlockTransactions.RUnlock()
	return calls
}

// MarkBlockAsDone calls MarkBlockAsDoneFunc.
func (mock *InterfaceMock) MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
	if mock.MarkBlockAsDoneFunc == nil {
		panic("InterfaceMock.MarkBlockAsDoneFunc: method is nil but Interface.MarkBlockAsDone was just called")
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
//	len(mockedInterface.MarkBlockAsDoneCalls())
func (mock *InterfaceMock) MarkBlockAsDoneCalls() []struct {
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

// OrphanHeight calls OrphanHeightFunc.
func (mock *InterfaceMock) OrphanHeight(ctx context.Context, height uint64) error {
	if mock.OrphanHeightFunc == nil {
		panic("InterfaceMock.OrphanHeightFunc: method is nil but Interface.OrphanHeight was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Height uint64
	}{
		Ctx:    ctx,
		Height: height,
	}
	mock.lockOrphanHeight.Lock()
	mock.calls.OrphanHeight = append(mock.calls.OrphanHeight, callInfo)
	mock.lockOrphanHeight.Unlock()
	return mock.OrphanHeightFunc(ctx, height)
}

// OrphanHeightCalls gets all the calls that were made to OrphanHeight.
// Check the length with:
//
//	len(mockedInterface.OrphanHeightCalls())
func (mock *InterfaceMock) OrphanHeightCalls() []struct {
	Ctx    context.Context
	Height uint64
} {
	var calls []struct {
		Ctx    context.Context
		Height uint64
	}
	mock.lockOrphanHeight.RLock()
	calls = mock.calls.OrphanHeight
	mock.lockOrphanHeight.RUnlock()
	return calls
}

// PrimaryBlocktx calls PrimaryBlocktxFunc.
func (mock *InterfaceMock) PrimaryBlocktx(ctx context.Context) (string, error) {
	if mock.PrimaryBlocktxFunc == nil {
		panic("InterfaceMock.PrimaryBlocktxFunc: method is nil but Interface.PrimaryBlocktx was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockPrimaryBlocktx.Lock()
	mock.calls.PrimaryBlocktx = append(mock.calls.PrimaryBlocktx, callInfo)
	mock.lockPrimaryBlocktx.Unlock()
	return mock.PrimaryBlocktxFunc(ctx)
}

// PrimaryBlocktxCalls gets all the calls that were made to PrimaryBlocktx.
// Check the length with:
//
//	len(mockedInterface.PrimaryBlocktxCalls())
func (mock *InterfaceMock) PrimaryBlocktxCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockPrimaryBlocktx.RLock()
	calls = mock.calls.PrimaryBlocktx
	mock.lockPrimaryBlocktx.RUnlock()
	return calls
}

// RegisterTransaction calls RegisterTransactionFunc.
func (mock *InterfaceMock) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (string, string, []byte, uint64, error) {
	if mock.RegisterTransactionFunc == nil {
		panic("InterfaceMock.RegisterTransactionFunc: method is nil but Interface.RegisterTransaction was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		Transaction *blocktx_api.TransactionAndSource
	}{
		Ctx:         ctx,
		Transaction: transaction,
	}
	mock.lockRegisterTransaction.Lock()
	mock.calls.RegisterTransaction = append(mock.calls.RegisterTransaction, callInfo)
	mock.lockRegisterTransaction.Unlock()
	return mock.RegisterTransactionFunc(ctx, transaction)
}

// RegisterTransactionCalls gets all the calls that were made to RegisterTransaction.
// Check the length with:
//
//	len(mockedInterface.RegisterTransactionCalls())
func (mock *InterfaceMock) RegisterTransactionCalls() []struct {
	Ctx         context.Context
	Transaction *blocktx_api.TransactionAndSource
} {
	var calls []struct {
		Ctx         context.Context
		Transaction *blocktx_api.TransactionAndSource
	}
	mock.lockRegisterTransaction.RLock()
	calls = mock.calls.RegisterTransaction
	mock.lockRegisterTransaction.RUnlock()
	return calls
}

// SetOrphanHeight calls SetOrphanHeightFunc.
func (mock *InterfaceMock) SetOrphanHeight(ctx context.Context, height uint64, orphaned bool) error {
	if mock.SetOrphanHeightFunc == nil {
		panic("InterfaceMock.SetOrphanHeightFunc: method is nil but Interface.SetOrphanHeight was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		Height   uint64
		Orphaned bool
	}{
		Ctx:      ctx,
		Height:   height,
		Orphaned: orphaned,
	}
	mock.lockSetOrphanHeight.Lock()
	mock.calls.SetOrphanHeight = append(mock.calls.SetOrphanHeight, callInfo)
	mock.lockSetOrphanHeight.Unlock()
	return mock.SetOrphanHeightFunc(ctx, height, orphaned)
}

// SetOrphanHeightCalls gets all the calls that were made to SetOrphanHeight.
// Check the length with:
//
//	len(mockedInterface.SetOrphanHeightCalls())
func (mock *InterfaceMock) SetOrphanHeightCalls() []struct {
	Ctx      context.Context
	Height   uint64
	Orphaned bool
} {
	var calls []struct {
		Ctx      context.Context
		Height   uint64
		Orphaned bool
	}
	mock.lockSetOrphanHeight.RLock()
	calls = mock.calls.SetOrphanHeight
	mock.lockSetOrphanHeight.RUnlock()
	return calls
}

// TryToBecomePrimary calls TryToBecomePrimaryFunc.
func (mock *InterfaceMock) TryToBecomePrimary(ctx context.Context, myHostName string) error {
	if mock.TryToBecomePrimaryFunc == nil {
		panic("InterfaceMock.TryToBecomePrimaryFunc: method is nil but Interface.TryToBecomePrimary was just called")
	}
	callInfo := struct {
		Ctx        context.Context
		MyHostName string
	}{
		Ctx:        ctx,
		MyHostName: myHostName,
	}
	mock.lockTryToBecomePrimary.Lock()
	mock.calls.TryToBecomePrimary = append(mock.calls.TryToBecomePrimary, callInfo)
	mock.lockTryToBecomePrimary.Unlock()
	return mock.TryToBecomePrimaryFunc(ctx, myHostName)
}

// TryToBecomePrimaryCalls gets all the calls that were made to TryToBecomePrimary.
// Check the length with:
//
//	len(mockedInterface.TryToBecomePrimaryCalls())
func (mock *InterfaceMock) TryToBecomePrimaryCalls() []struct {
	Ctx        context.Context
	MyHostName string
} {
	var calls []struct {
		Ctx        context.Context
		MyHostName string
	}
	mock.lockTryToBecomePrimary.RLock()
	calls = mock.calls.TryToBecomePrimary
	mock.lockTryToBecomePrimary.RUnlock()
	return calls
}
