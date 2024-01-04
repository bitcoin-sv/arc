package store

import (
	"context"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/pkg/errors"
)

var (
	ErrNotFound = errors.New("not found")

	// ErrBlockNotFound is returned when a block is not found.
	ErrBlockNotFound = errors.New("block not found")

	// ErrTransactionNotFound is returned when a transaction is not found.
	ErrTransactionNotFound = errors.New("transaction not found")
)

type Interface interface {
	RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (string, string, []byte, uint64, error)
	TryToBecomePrimary(ctx context.Context, myHostName string) error
	PrimaryBlocktx(ctx context.Context) (string, error)
	GetTransactionMerklePath(ctx context.Context, hash *chainhash.Hash) (string, error)
	GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error)
	GetBlockForHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error)
	GetBlockTransactions(ctx context.Context, block *blocktx_api.Block) (*blocktx_api.Transactions, error)
	GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error)
	GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error)
	GetTransactionBlocks(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error)
	InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error)
	InsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) error
	MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error
	OrphanHeight(ctx context.Context, height uint64) error
	SetOrphanHeight(ctx context.Context, height uint64, orphaned bool) error
	GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error)
	GetBlockGaps(ctx context.Context) ([]*BlockGap, error)
	Close() error
}
