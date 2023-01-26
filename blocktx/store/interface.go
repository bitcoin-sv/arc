package store

import (
	"context"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
)

type Interface interface {
	InsertTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error
	GetTransactionSource(ctx context.Context, txid []byte) (string, error)
	GetBlock(ctx context.Context, hash []byte) (*blocktx_api.Block, error)
	GetBlockForHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error)
	GetBlockTransactions(ctx context.Context, block *blocktx_api.Block) (*blocktx_api.Transactions, error)
	GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error)
	GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error)
	GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Blocks, error)
	InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error)
	InsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource) error
	MarkBlockAsDone(ctx context.Context, hash []byte, size uint64, txCount uint64) error
	OrphanHeight(ctx context.Context, height uint64) error
	SetOrphanHeight(ctx context.Context, height uint64, orphaned bool) error
	GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error)
	Close() error
}
