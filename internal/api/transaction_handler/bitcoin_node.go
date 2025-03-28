package transaction_handler

import (
	"context"
	"encoding/hex"

	"github.com/ordishs/go-bitcoin"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
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

// GetTransactions gets a raw transactions from the bitcoin node.
func (b *BitcoinNode) GetTransactions(_ context.Context, txIDs []string) ([]*metamorph.Transaction, error) {
	txs := make([]*metamorph.Transaction, 0, len(txIDs))

	for _, id := range txIDs {
		rawTx, err := b.Node.GetRawTransaction(id)
		if err != nil {
			return nil, err
		}
		txBytes, err := hex.DecodeString(rawTx.Hex)
		if err != nil {
			return nil, err
		}

		mtx := metamorph.Transaction{
			TxID:        rawTx.TxID,
			BlockHeight: rawTx.BlockHeight,
			Bytes:       txBytes,
		}
		txs = append(txs, &mtx)
	}

	return txs, nil
}

func (b *BitcoinNode) Health(_ context.Context) error {
	return nil
}

// GetTransactionStatus gets a raw transaction from the bitcoin node.
func (b *BitcoinNode) GetTransactionStatus(_ context.Context, txID string) (status *metamorph.TransactionStatus, err error) {
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

	return &metamorph.TransactionStatus{
		TxID:        txID,
		BlockHash:   rawTx.BlockHash,
		BlockHeight: rawTx.BlockHeight,
		Status:      statusText,
	}, nil
}

func (b *BitcoinNode) GetTransactionStatuses(_ context.Context, _ []string) (statuses []*metamorph.TransactionStatus, err error) {
	return make([]*metamorph.TransactionStatus, 0), nil
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format.
func (b *BitcoinNode) SubmitTransaction(_ context.Context, tx *sdkTx.Transaction, _ *metamorph.TransactionOptions) (*metamorph.TransactionStatus, error) {
	txID, err := b.Node.SendRawTransaction(hex.EncodeToString(tx.Bytes()))
	if err != nil {
		return nil, err
	}

	var rawTx *bitcoin.RawTransaction
	rawTx, err = b.Node.GetRawTransaction(txID)
	if err != nil {
		return nil, err
	}

	return &metamorph.TransactionStatus{
		BlockHash:   rawTx.BlockHash,
		BlockHeight: rawTx.BlockHeight,
		Status:      metamorph_api.Status_SENT_TO_NETWORK.String(),
		TxID:        txID,
	}, nil
}

// SubmitTransactions submits a list of transaction to the bitcoin network and returns the transaction statuses.
func (b *BitcoinNode) SubmitTransactions(_ context.Context, txs sdkTx.Transactions, _ *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
	statuses := make([]*metamorph.TransactionStatus, 0, len(txs))
	for _, tx := range txs {
		txID, err := b.Node.SendRawTransaction(hex.EncodeToString(tx.Bytes()))
		if err != nil {
			return nil, err
		}

		var rawTx *bitcoin.RawTransaction
		rawTx, err = b.Node.GetRawTransaction(txID)
		if err != nil {
			return nil, err
		}

		statuses = append(statuses, &metamorph.TransactionStatus{
			BlockHash:   rawTx.BlockHash,
			BlockHeight: rawTx.BlockHeight,
			Status:      metamorph_api.Status_SENT_TO_NETWORK.String(),
			TxID:        txID,
		})
	}

	return statuses, nil
}
