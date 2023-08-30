package transactionHandler

import (
	"context"

	arc "github.com/bitcoin-sv/arc/api"
	"github.com/pkg/errors"
)

var (
	// ErrTransactionNotFound is returned when a transaction is not found.
	ErrTransactionNotFound = errors.New("transaction not found")

	// ErrParentTransactionNotFound is returned when a parent transaction is not found.
	ErrParentTransactionNotFound = errors.New("parent transaction not found")
)

//go:generate moq -pkg test -out ../test/transactionHandler_mock.go . TransactionHandler
type TransactionHandler interface {
	GetTransaction(ctx context.Context, txID string) ([]byte, error)
	GetTransactionStatus(ctx context.Context, txID string) (*TransactionStatus, error)
	SubmitTransaction(ctx context.Context, tx []byte, options *arc.TransactionOptions) (*TransactionStatus, error)
	SubmitTransactions(ctx context.Context, tx [][]byte, options *arc.TransactionOptions) ([]*TransactionStatus, error)
}

// TransactionStatus defines model for TransactionStatus.
type TransactionStatus struct {
	TxID        string `json:"tx_id"`
	BlockHash   string `json:"blockHash,omitempty"`
	BlockHeight uint64 `json:"blockHeight,omitempty"`
	Status      string `json:"status"`
	TxStatus    string `json:"txStatus"`
	ExtraInfo   string `json:"extraInfo,omitempty"`
	Timestamp   int64  `json:"timestamp"`
}
