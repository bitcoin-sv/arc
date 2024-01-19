package transaction_handler

import (
	"context"
	"encoding/hex"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/ordishs/go-bitcoin"
)

// BitcoinNode is a real Bitcoin node.
type BitcoinNode struct {
	Node *bitcoin.Bitcoind
}

// NewBitcoinNode creates a connection to a real bitcoin node via RPC.
func NewBitcoinNode(host string, port int, user, passwd string, useSSL bool) (*BitcoinNode, error) {
	node, err := bitcoin.New(host, port, user, passwd, useSSL)
	if err != nil {
		return nil, err
	}

	return &BitcoinNode{
		Node: node,
	}, nil
}

// GetTransaction gets a raw transaction from the bitcoin node.
func (b *BitcoinNode) GetTransaction(_ context.Context, txID string) (txBytes []byte, err error) {
	var rawTx *bitcoin.RawTransaction
	rawTx, err = b.Node.GetRawTransaction(txID)
	if err != nil {
		return nil, err
	}

	txBytes, err = hex.DecodeString(rawTx.Hex)
	if err != nil {
		return nil, err
	}

	return txBytes, nil
}

// GetTransactionStatus gets a raw transaction from the bitcoin node.
func (b *BitcoinNode) GetTransactionStatus(_ context.Context, txID string) (status *TransactionStatus, err error) {
	var rawTx *bitcoin.RawTransaction
	rawTx, err = b.Node.GetRawTransaction(txID)
	if err != nil {
		return nil, err
	}

	// if we get here, we have a raw transaction, and it has been seen on the network
	statusText := metamorph_api.Status_SEEN_ON_NETWORK.String()
	if rawTx.BlockHash != "" {
		statusText = metamorph_api.Status_MINED.String()
	}

	return &TransactionStatus{
		TxID:        txID,
		BlockHash:   rawTx.BlockHash,
		BlockHeight: rawTx.BlockHeight,
		Status:      statusText,
	}, nil
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format.
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
		Status:      metamorph_api.Status_SENT_TO_NETWORK.String(),
		TxID:        txID,
	}, nil
}

// SubmitTransactions submits a list of transaction to the bitcoin network and returns the transaction statuses.
func (b *BitcoinNode) SubmitTransactions(_ context.Context, txs [][]byte, _ *api.TransactionOptions) ([]*TransactionStatus, error) {
	statuses := make([]*TransactionStatus, 0, len(txs))
	for _, tx := range txs {
		txID, err := b.Node.SendRawTransaction(hex.EncodeToString(tx))
		if err != nil {
			return nil, err
		}

		var rawTx *bitcoin.RawTransaction
		rawTx, err = b.Node.GetRawTransaction(txID)
		if err != nil {
			return nil, err
		}

		statuses = append(statuses, &TransactionStatus{
			BlockHash:   rawTx.BlockHash,
			BlockHeight: rawTx.BlockHeight,
			Status:      metamorph_api.Status_SENT_TO_NETWORK.String(),
			TxID:        txID,
		})
	}

	return statuses, nil
}
