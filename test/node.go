package test

import (
	"github.com/ordishs/go-bitcoin"
)

type Node struct {
	GetRawTransactionResult  []interface{}
	SendRawTransactionResult []interface{}
}

func (n *Node) GetRawTransaction(_ string) (rawTx *bitcoin.RawTransaction, err error) {
	if n.GetRawTransactionResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		result, n.GetRawTransactionResult = n.GetRawTransactionResult[0], n.GetRawTransactionResult[1:]

		switch r := result.(type) {
		case error:
			return nil, r
		case *bitcoin.RawTransaction:
			return r, nil
		}
	}

	return nil, nil
}

func (n *Node) SendRawTransaction(_ string) (txID string, err error) {
	if n.SendRawTransactionResult != nil {
		var result interface{}
		// pop the first result of the stack and return it
		result, n.SendRawTransactionResult = n.SendRawTransactionResult[0], n.SendRawTransactionResult[1:]

		switch r := result.(type) {
		case error:
			return "", r
		case string:
			return r, nil
		}
	}

	return "", nil
}
