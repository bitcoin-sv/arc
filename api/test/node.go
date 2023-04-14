package test

import (
	"context"

	arc "github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
)

type SubmitTransactionRequest struct {
	Transaction []byte
	Options     *arc.TransactionOptions
}

type Node struct {
	GetTransactionResult       []interface{}
	GetTransactionStatusResult []interface{}
	SubmitTransactionRequests  []*SubmitTransactionRequest
	SubmitTransactionResult    []interface{}
}

func (n *Node) SubmitTransaction(_ context.Context, txBytes []byte, options *arc.TransactionOptions) (rawTx *transactionHandler.TransactionStatus, err error) {
	n.SubmitTransactionRequests = append(n.SubmitTransactionRequests, &SubmitTransactionRequest{
		Transaction: txBytes,
		Options:     options,
	})

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
		if len(n.GetTransactionResult) > 1 {
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
