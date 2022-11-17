package client

import (
	"context"

	"github.com/TAAL-GmbH/mapi"
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
func (b *BitcoinNode) GetTransaction(_ context.Context, txID string) (rawTx *bitcoin.RawTransaction, err error) {
	return b.Node.GetRawTransaction(txID) //nolint:contextcheck - no context
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format
func (b *BitcoinNode) SubmitTransaction(_ context.Context, tx string, _ *mapi.TransactionOptions) (*bitcoin.RawTransaction, error) {
	txID, err := b.Node.SendRawTransaction(tx) //nolint:contextcheck - no context
	if err != nil {
		return nil, err
	}

	return b.Node.GetRawTransaction(txID) //nolint:contextcheck - no context
}
