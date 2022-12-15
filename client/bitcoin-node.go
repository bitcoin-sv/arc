package client

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/ordishs/go-bitcoin"
)

// BitcoinNode is a real Bitcoin node
type BitcoinNode struct {
	Node *bitcoin.Bitcoind
}

// NewBitcoinNode creates a connection to a real bitcoin node via RPC
func NewBitcoinNode(host string, port int, user, passwd string, useSSL bool) (*BitcoinNode, error) {
	node, err := bitcoin.New(host, port, user, passwd, useSSL)
	if err != nil {
		return nil, err
	}

	return &BitcoinNode{
		Node: node,
	}, nil
}

// GetTransaction gets a raw transaction from the bitcoin node
func (b *BitcoinNode) GetTransaction(_ context.Context, txID string) (rawTx *RawTransaction, err error) {
	var bRawTx *bitcoin.RawTransaction
	if bRawTx, err = b.Node.GetRawTransaction(txID); err != nil {
		return nil, err
	}

	var jb []byte
	if jb, err = json.Marshal(bRawTx); err != nil {
		return nil, err
	}

	err = json.Unmarshal(jb, &rawTx)
	return rawTx, err
}

// GetTransactionStatus gets a raw transaction from the bitcoin node
func (b *BitcoinNode) GetTransactionStatus(_ context.Context, txID string) (status *TransactionStatus, err error) {
	var rawTx *bitcoin.RawTransaction
	rawTx, err = b.Node.GetRawTransaction(txID)
	if err != nil {
		return nil, err
	}

	return &TransactionStatus{
		BlockHash:   rawTx.BlockHash,
		BlockHeight: rawTx.BlockHeight,
		Status:      "",
		TxID:        txID,
	}, nil
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format
func (b *BitcoinNode) SubmitTransaction(_ context.Context, tx []byte, _ *api.TransactionOptions) (*TransactionStatus, error) {
	txID, err := b.Node.SendRawTransaction(hex.EncodeToString(tx))
	if err != nil {
		return nil, err
	}

	var rawTx *bitcoin.RawTransaction
	rawTx, err = b.Node.GetRawTransaction(txID)
	if err != nil {
		return nil, err
	}

	return &TransactionStatus{
		BlockHash:   rawTx.BlockHash,
		BlockHeight: rawTx.BlockHeight,
		Status:      "",
		TxID:        txID,
	}, nil
}
