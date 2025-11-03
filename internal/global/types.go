package global

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
)

var ErrTransactionNotFound = errors.New("transaction not found")

const (
	MaxTimeout = 30
)

// MerkleRootsVerifier verifies the merkle roots existence in blocktx db and returns unverified block heights.
type MerkleRootsVerifier interface {
	// VerifyMerkleRoots verifies the merkle roots existence in blocktx db and returns unverified block heights.
	VerifyMerkleRoots(ctx context.Context, merkleRootVerificationRequest []blocktx.MerkleRootVerificationRequest) ([]uint64, error)
}

type TransactionHandler interface {
	Health(ctx context.Context) error
	GetTransactions(ctx context.Context, txIDs []string) ([]*Transaction, error)
	GetTransactionStatus(ctx context.Context, txID string) (*TransactionStatus, error)
	GetTransactionStatuses(ctx context.Context, txIDs []string) ([]*TransactionStatus, error)
	SubmitTransactions(ctx context.Context, tx sdkTx.Transactions, options *TransactionOptions) ([]*TransactionStatus, error)
}

type BlocktxClient interface {
	AnyTransactionsMined(ctx context.Context, hash [][]byte) ([]*blocktx_api.IsMined, error)
	RegisterTransaction(ctx context.Context, hash []byte) error
	RegisterTransactions(ctx context.Context, hashes [][]byte) error
	CurrentBlockHeight(ctx context.Context) (*blocktx_api.CurrentBlockHeightResponse, error)
	LatestBlocks(ctx context.Context, blocks uint64) (*blocktx_api.LatestBlocksResponse, error)
}

// TransactionStatus defines model for TransactionStatus.
type TransactionStatus struct {
	TxID          string
	MerklePath    string
	BlockHash     string
	BlockHeight   uint64
	Status        string
	ExtraInfo     string
	Callbacks     []*metamorph_api.Callback
	CompetingTxs  []string
	LastSubmitted time.Time
	Timestamp     int64
}

type Transaction struct {
	TxID        string
	Bytes       []byte
	BlockHeight uint64
}

// TransactionOptions options passed from the header when creating transactions.
type TransactionOptions struct {
	CallbackURL             string
	CallbackToken           string
	CallbackBatch           bool
	SkipFeeValidation       bool
	SkipScriptValidation    bool
	SkipTxValidation        bool
	ForceValidation         bool
	CumulativeFeeValidation bool
	WaitForStatus           metamorph_api.Status
	FullStatusUpdates       bool
}

type Stoppable interface {
	Shutdown()
}

type StoppableWithError interface {
	Shutdown() error
}

type StoppableWithContext interface {
	Shutdown(ctx context.Context) error
}

type Stoppables []Stoppable
type StoppablesWithError []StoppableWithError

type StoppablesWithContext []StoppableWithContext

func (s Stoppables) Shutdown() {
	for _, stoppable := range s {
		if stoppable != nil {
			stoppable.Shutdown()
		}
	}
}

func (s StoppablesWithError) Shutdown(logger *slog.Logger) {
	for _, stoppable := range s {
		if stoppable != nil {
			err := stoppable.Shutdown()
			if err != nil {
				logger.Error("Error shutting down stoppable", slog.String("err", err.Error()))
			}
		}
	}
}

func (s StoppablesWithContext) Shutdown(ctx context.Context, logger *slog.Logger) {
	for _, stoppable := range s {
		if stoppable != nil {
			err := stoppable.Shutdown(ctx)
			if err != nil {
				logger.Error("Error shutting down stoppable", slog.String("err", err.Error()))
			}
		}
	}
}
