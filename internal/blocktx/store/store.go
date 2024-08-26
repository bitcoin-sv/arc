package store

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

var (
	ErrNotFound                    = errors.New("not found")
	ErrBlockProcessingDuplicateKey = errors.New("block hash already exists")
	ErrBlockNotFound               = errors.New("block not found")
)

type BlocktxStore interface {
	RegisterTransactions(ctx context.Context, transaction []*blocktx_api.TransactionAndSource) (updatedTxs []*chainhash.Hash, err error)
	GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error)
	GetChainTip(ctx context.Context) (*blocktx_api.Block, error)
	InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error)
	UpsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) (registeredTxs []UpsertBlockTransactionsResult, err error)
	MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error
	GetBlockGaps(ctx context.Context, heightRange int) ([]*BlockGap, error)
	ClearBlocktxTable(ctx context.Context, retentionDays int32, table string) (*blocktx_api.RowsAffectedResponse, error)
	GetMinedTransactions(ctx context.Context, hashes []*chainhash.Hash) ([]GetMinedTransactionResult, error)

	SetBlockProcessing(ctx context.Context, hash *chainhash.Hash, processedBy string) (string, error)
	GetBlockHashesProcessingInProgress(ctx context.Context, processedBy string) ([]*chainhash.Hash, error)
	DelBlockProcessing(ctx context.Context, hash *chainhash.Hash, processedBy string) (int64, error)
	VerifyMerkleRoots(ctx context.Context, merkleRoots []*blocktx_api.MerkleRootVerificationRequest, maxAllowedBlockHeightMismatch int) (*blocktx_api.MerkleRootVerificationResponse, error)

	Ping(ctx context.Context) error
	Close() error
}
