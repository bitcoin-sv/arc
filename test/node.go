package test

import (
	"context"

	arc "github.com/TAAL-GmbH/arc"
	"github.com/TAAL-GmbH/arc/client"
)

type Node struct {
	GetRawTransactionResult  []interface{}
	SendRawTransactionResult []interface{}
}

func (n *Node) GetTransaction(_ context.Context, _ string) (rawTx *client.RawTransaction, err error) {
	if n.GetRawTransactionResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		result, n.GetRawTransactionResult = n.GetRawTransactionResult[0], n.GetRawTransactionResult[1:]

		switch r := result.(type) {
		case error:
			return nil, r
		case *client.RawTransaction:
			return r, nil
		}
	}

	return nil, nil
}

func (n *Node) SubmitTransaction(_ context.Context, _ []byte, _ *arc.TransactionOptions) (rawTx *client.TransactionStatus, err error) {
	if n.SendRawTransactionResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		result, n.SendRawTransactionResult = n.SendRawTransactionResult[0], n.SendRawTransactionResult[1:]

		switch r := result.(type) {
		case error:
			return nil, r
		case *client.TransactionStatus:
			return r, nil
		}
	}

	return nil, nil
}

func (b *Node) GetTransactionStatus(_ context.Context, txID string) (status *client.TransactionStatus, err error) {
	return
}
