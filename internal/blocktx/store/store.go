package store

import (
	"context"
	"errors"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

var (
	ErrNotFound                      = errors.New("not found")
	ErrBlockProcessingMaximumReached = errors.New("block processing maximum reached")
	ErrBlockProcessingInProgress     = errors.New("block processing already in progress")
	ErrBlockNotFound                 = errors.New("block not found")
	ErrUnableToPrepareStatement      = errors.New("unable to prepare statement")
	ErrUnableToDeleteRows            = errors.New("unable to delete rows")
	ErrUnableToGetSQLConnection      = errors.New("unable to get or create sql connection")
	ErrFailedToInsertBlock           = errors.New("failed to insert block")
	ErrFailedToUpdateBlockStatuses   = errors.New("failed to update block statuses")
	ErrFailedToOpenDB                = errors.New("failed to open postgres database")
	ErrFailedToInsertTransactions    = errors.New("failed to bulk insert transactions")
	ErrFailedToGetRows               = errors.New("failed to get rows")
	ErrFailedToSetBlockProcessing    = errors.New("failed to set block processing")
	ErrFailedToParseHash             = errors.New("failed to parse hash")
)

type Stats struct {
	CurrentNumOfBlockGaps int64
}

type BlocktxStore interface {
	RegisterTransactions(ctx context.Context, txHashes [][]byte) (rowsAffected int64, err error)
	GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error)
	GetLongestBlockByHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error)
	GetChainTip(ctx context.Context) (*blocktx_api.Block, error)
	UpsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error)
	InsertBlockTransactions(ctx context.Context, blockID uint64, txsWithMerklePaths []TxHashWithMerkleTreeIndex) error
	MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error
	GetBlockGaps(ctx context.Context, heightRange int) ([]*BlockGap, error)
	ClearBlocktxTable(ctx context.Context, retentionDays int32, table string) (*blocktx_api.RowsAffectedResponse, error)
	GetMinedTransactions(ctx context.Context, hashes [][]byte) ([]BlockTransaction, error)
	GetLongestChainFromHeight(ctx context.Context, height uint64) ([]*blocktx_api.Block, error)
	GetStaleChainBackFromHash(ctx context.Context, hash []byte) ([]*blocktx_api.Block, error)
	GetOrphansForwardFromHash(ctx context.Context, hash []byte) ([]*blocktx_api.Block, error)
	GetOrphansBackToNonOrphanAncestor(ctx context.Context, hash []byte) (orphans []*blocktx_api.Block, nonOrphanAncestor *blocktx_api.Block, err error)
	GetRegisteredTxsByBlockHashes(ctx context.Context, blockHashes [][]byte) ([]BlockTransaction, error)
	GetBlockTransactionsHashes(ctx context.Context, blockHash []byte) ([]*chainhash.Hash, error)
	UpdateBlocksStatuses(ctx context.Context, blockStatusUpdates []BlockStatusUpdate) error
	GetStats(ctx context.Context) (*Stats, error)

	SetBlockProcessing(ctx context.Context, hash *chainhash.Hash, setProcessedBy string, lockTime time.Duration, maxParallelProcessing int) (string, error)
	VerifyMerkleRoots(ctx context.Context, merkleRoots []*blocktx_api.MerkleRootVerificationRequest, maxAllowedBlockHeightMismatch uint64) (*blocktx_api.MerkleRootVerificationResponse, error)

	Ping(ctx context.Context) error
	Close() error
}
