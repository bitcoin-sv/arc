package global

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	LastSubmitted timestamppb.Timestamp
	Timestamp     int64
}

type Transaction struct {
	TxID        string
	Bytes       []byte
	BlockHeight uint64
}

// TransactionOptions options passed from header when creating transactions.
type TransactionOptions struct {
	CallbackURL             string               `json:"callback_url,omitempty"`
	CallbackToken           string               `json:"callback_token,omitempty"`
	CallbackBatch           bool                 `json:"callback_batch,omitempty"`
	SkipFeeValidation       bool                 `json:"X-SkipFeeValidation,omitempty"`
	SkipScriptValidation    bool                 `json:"X-SkipScriptValidation,omitempty"`
	SkipTxValidation        bool                 `json:"X-SkipTxValidation,omitempty"`
	ForceValidation         bool                 `json:"X-ForceValidation,omitempty"`
	CumulativeFeeValidation bool                 `json:"X-CumulativeFeeValidation,omitempty"`
	WaitForStatus           metamorph_api.Status `json:"wait_for_status,omitempty"`
	FullStatusUpdates       bool                 `json:"full_status_updates,omitempty"`
}
