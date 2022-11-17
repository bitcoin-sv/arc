package client

import (
	"context"

	"github.com/TAAL-GmbH/mapi"
	"github.com/ordishs/go-bitcoin"
)

type TransactionHandler interface {
	GetTransaction(ctx context.Context, txID string) (rawTx *bitcoin.RawTransaction, err error)
	SubmitTransaction(ctx context.Context, tx string, options *mapi.TransactionOptions) (rawTx *bitcoin.RawTransaction, err error)
}
