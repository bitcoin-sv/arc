package test

import (
	"context"

	arc "github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
)

type Node struct {
	GetRawTransactionResult  []interface{}
	SendRawTransactionResult []interface{}
}

func (n *Node) GetTransaction(_ context.Context, _ string) (rawTx *transactionHandler.RawTransaction, err error) {
	if n.GetRawTransactionResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		result, n.GetRawTransactionResult = n.GetRawTransactionResult[0], n.GetRawTransactionResult[1:]

		switch r := result.(type) {
		case error:
			return nil, r
		case *transactionHandler.RawTransaction:
			return r, nil
		}
	}

	return nil, nil
}

func (n *Node) SubmitTransaction(_ context.Context, _ []byte, _ *arc.TransactionOptions) (rawTx *transactionHandler.TransactionStatus, err error) {
	if n.SendRawTransactionResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		result, n.SendRawTransactionResult = n.SendRawTransactionResult[0], n.SendRawTransactionResult[1:]

		switch r := result.(type) {
		case error:
			return nil, r
		case *transactionHandler.TransactionStatus:
			return r, nil
		}
	}

	return nil, nil
}

func (n *Node) GetTransactionStatus(_ context.Context, txID string) (status *transactionHandler.TransactionStatus, err error) {
	return
}
