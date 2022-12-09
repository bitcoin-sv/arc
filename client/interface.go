package client

import (
	"context"

	arc "github.com/TAAL-GmbH/arc"
	"github.com/ordishs/go-bitcoin"
)

type TransactionHandler interface {
	GetTransaction(ctx context.Context, txID string) (*RawTransaction, error)
	GetTransactionStatus(ctx context.Context, txID string) (*TransactionStatus, error)
	SubmitTransaction(ctx context.Context, tx []byte, options *arc.TransactionOptions) (*TransactionStatus, error)
}

type RawTransaction struct {
	bitcoin.RawTransaction
	Status string `json:"status"`
}

// TransactionStatus defines model for TransactionStatus.
type TransactionStatus struct {
	TxID        string `json:"tx_id"`
	BlockHash   string `json:"blockHash,omitempty"`
	BlockHeight uint64 `json:"blockHeight,omitempty"`
	Status      string `json:"status"`
	Timestamp   int64  `json:"timestamp"`
}
