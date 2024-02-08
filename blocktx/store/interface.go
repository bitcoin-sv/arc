package store

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

var (
	ErrNotFound = errors.New("not found")

	// ErrBlockNotFound is returned when a block is not found.
	ErrBlockNotFound = errors.New("block not found")
)

type Interface interface {
	RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error
	TryToBecomePrimary(ctx context.Context, myHostName string) error
	GetPrimary(ctx context.Context) (string, error)
	GetTransactionMerklePath(ctx context.Context, hash *chainhash.Hash) (string, error)
	GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error)
	GetTransactionBlocks(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error)
	InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error)
	UpdateBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) error
	MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error
	GetBlockGaps(ctx context.Context, heightRange int) ([]*BlockGap, error)
	Close() error
}
