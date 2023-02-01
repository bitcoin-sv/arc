package test

import (
	"context"

	arc "github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
)

type Node struct {
	GetTransactionResult       []interface{}
	GetTransactionStatusResult []interface{}
	SubmitTransactionResult    []interface{}
}

func (n *Node) SubmitTransaction(_ context.Context, _ []byte, _ *arc.TransactionOptions) (rawTx *transactionHandler.TransactionStatus, err error) {
	if n.SubmitTransactionResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		if len(n.SubmitTransactionResult) > 1 {
			result, n.SubmitTransactionResult = n.SubmitTransactionResult[0], n.SubmitTransactionResult[1:]
		} else {
			result, n.SubmitTransactionResult = n.SubmitTransactionResult[0], nil
		}

		switch r := result.(type) {
		case error:
			return nil, r
		case *transactionHandler.TransactionStatus:
			return r, nil
		}
	}

	return nil, nil
}

func (n *Node) GetTransaction(_ context.Context, txID string) ([]byte, error) {
	if n.GetTransactionResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		if len(n.SubmitTransactionResult) > 1 {
			result, n.GetTransactionResult = n.GetTransactionResult[0], n.GetTransactionResult[1:]
		} else {
			result, n.GetTransactionResult = n.GetTransactionResult[0], nil
		}

		switch r := result.(type) {
		case error:
			return nil, r
		case []byte:
			return r, nil
		}
	}

	return nil, nil
}

func (n *Node) GetTransactionStatus(_ context.Context, txID string) (*transactionHandler.TransactionStatus, error) {
	if n.GetTransactionStatusResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		if len(n.SubmitTransactionResult) > 1 {
			result, n.GetTransactionStatusResult = n.GetTransactionStatusResult[0], n.GetTransactionStatusResult[1:]
		} else {
			result, n.GetTransactionStatusResult = n.GetTransactionStatusResult[0], nil
		}

		switch r := result.(type) {
		case error:
			return nil, r
		case *transactionHandler.TransactionStatus:
			return r, nil
		}
	}

	return nil, nil
}
