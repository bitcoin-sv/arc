package global

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bsv-blockchain/go-bt/v2/chainhash"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/ccoveille/go-safecast"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrNotFound            = errors.New("key could not be found")
	ErrUpdateCompeting     = fmt.Errorf("failed to updated competing transactions with status %s", metamorph_api.Status_REJECTED.String())
)

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

type Client interface {
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

type Data struct {
	RawTx             []byte
	StoredAt          time.Time
	LastModified      time.Time
	Hash              *chainhash.Hash
	Status            metamorph_api.Status
	BlockHeight       uint64
	BlockHash         *chainhash.Hash
	Callbacks         []Callback
	StatusHistory     []*StatusWithTimestamp
	FullStatusUpdates bool
	RejectReason      string
	CompetingTxs      []string
	LockedBy          string
	TTL               int64
	MerklePath        string
	LastSubmittedAt   time.Time
	Retries           int
}

type Callback struct {
	CallbackURL   string `json:"callback_url"`
	CallbackToken string `json:"callback_token"`
	AllowBatch    bool   `json:"allow_batch"`
}

type StatusWithTimestamp struct {
	Status    metamorph_api.Status `json:"status"`
	Timestamp time.Time            `json:"timestamp"`
}

func (d *Data) UpdateStatusFromSQL(status sql.NullInt32) {
	if status.Valid {
		d.Status = metamorph_api.Status(status.Int32)
	}
}

func (d *Data) UpdateBlockHash(blockHash []byte) error {
	if len(blockHash) > 0 {
		var err error
		d.BlockHash, err = chainhash.NewHash(blockHash)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Data) UpdateTxHash(txHash []byte) error {
	if len(txHash) > 0 {
		var err error
		d.Hash, err = chainhash.NewHash(txHash)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Data) UpdateBlockHeightFromSQL(blockHeight sql.NullInt64) error {
	blockHeightUint64, err := safecast.ToUint64(blockHeight.Int64)
	if err != nil {
		return err
	}
	if blockHeight.Valid {
		d.BlockHeight = blockHeightUint64
	}
	return nil
}

func (d *Data) UpdateRetriesFromSQL(retries sql.NullInt32) {
	if retries.Valid {
		d.Retries = int(retries.Int32)
	}
}

func (d *Data) UpdateLastModifiedFromSQL(lastModified sql.NullTime) {
	if lastModified.Valid {
		d.LastModified = lastModified.Time.UTC()
	}
}

func (d *Data) UpdateCompetingTxs(competingTxs sql.NullString) {
	if competingTxs.String != "" {
		d.CompetingTxs = strings.Split(competingTxs.String, ",")
	}
}
