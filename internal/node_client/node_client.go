package node_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/tracing"
)

var (
	ErrFailedToGetRawTransaction   = errors.New("failed to get raw transaction")
	ErrFailedToGetMempoolAncestors = errors.New("failed to get mempool ancestors")
	ErrTransactionNotFound         = errors.New("transaction not found")
)

type NodeClient struct {
	bitcoinClient     *bitcoin.Bitcoind
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

type NodeClientI interface {
	GetMempoolAncestors(ctx context.Context, ids []string) ([]string, error)
	GetRawTransaction(ctx context.Context, id string) (*sdkTx.Transaction, error)
	GetTransactionInfo(ctx context.Context, id string) (rt *Transaction, err error)
}

type Transaction struct {
	Hex           string `json:"hex,omitempty"`
	TxID          string `json:"txid"`
	Hash          string `json:"hash"`
	Version       int32  `json:"version"`
	Size          uint32 `json:"size"`
	LockTime      uint32 `json:"locktime"`
	BlockHash     string `json:"blockhash,omitempty"`
	Confirmations uint32 `json:"confirmations,omitempty"`
	Time          int64  `json:"time,omitempty"`
	Blocktime     int64  `json:"blocktime,omitempty"`
	BlockHeight   uint64 `json:"blockheight,omitempty"`
}

func WithTracer(attr ...attribute.KeyValue) func(s *NodeClient) {
	return func(p *NodeClient) {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}
	}
}

func New(n *bitcoin.Bitcoind, opts ...func(client *NodeClient)) (NodeClient, error) {
	node := NodeClient{
		bitcoinClient: n,
	}

	for _, opt := range opts {
		opt(&node)
	}

	return node, nil
}

func (n NodeClient) GetMempoolAncestors(ctx context.Context, ids []string) (allTxIDs []string, err error) {
	_, span := tracing.StartTracing(ctx, "NodeClient_GetMempoolAncestors", n.tracingEnabled, n.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	uniqueIDs := make(map[string]struct{})

	for _, id := range ids {
		_, getMemPoolAncSpan := tracing.StartTracing(ctx, "Bitcoind_GetMempoolAncestors", n.tracingEnabled, n.tracingAttributes...)
		var nTx []byte
		nTx, err = n.bitcoinClient.GetMempoolAncestors(id, false)
		tracing.EndTracing(getMemPoolAncSpan, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Transaction not in mempool") {
				continue
			}

			return nil, errors.Join(ErrFailedToGetMempoolAncestors, err)
		}

		if nTx == nil {
			return nil, nil
		}

		var txIDs []string

		err = json.Unmarshal(nTx, &txIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw transaction: %v", err)
		}

		for _, txID := range txIDs {
			uniqueIDs[txID] = struct{}{}
		}

		uniqueIDs[id] = struct{}{}
	}

	allTxIDs = make([]string, len(uniqueIDs))
	counter := 0
	for id := range uniqueIDs {
		allTxIDs[counter] = id
		counter++
	}
	return allTxIDs, nil
}

func (n NodeClient) GetRawTransaction(ctx context.Context, id string) (rt *sdkTx.Transaction, err error) {
	_, span := tracing.StartTracing(ctx, "NodeClient_GetRawTransaction", n.tracingEnabled, n.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	nTx, err := n.bitcoinClient.GetRawTransaction(id)
	if err != nil {
		if strings.Contains(err.Error(), "No such mempool or blockchain transaction") {
			return nil, errors.Join(ErrTransactionNotFound, err)
		}
		return nil, errors.Join(ErrFailedToGetRawTransaction, err)
	}

	rt, err = sdkTx.NewTransactionFromHex(nTx.Hex)
	if err != nil {
		return nil, err
	}

	return rt, nil
}

func (n NodeClient) GetTransactionInfo(ctx context.Context, id string) (rt *Transaction, err error) {
	_, span := tracing.StartTracing(ctx, "NodeClient_GetRawTransaction", n.tracingEnabled, n.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	nTx, err := n.bitcoinClient.GetRawTransaction(id)
	if err != nil {
		if strings.Contains(err.Error(), "No such mempool or blockchain transaction") {
			return nil, errors.Join(ErrTransactionNotFound, err)
		}
		return nil, errors.Join(ErrFailedToGetRawTransaction, err)
	}

	rt = &Transaction{
		Hex:           nTx.Hex,
		TxID:          nTx.TxID,
		Hash:          nTx.Hash,
		Version:       nTx.Version,
		Size:          nTx.Size,
		LockTime:      nTx.LockTime,
		BlockHash:     nTx.BlockHash,
		Confirmations: nTx.Confirmations,
		Time:          nTx.Time,
		Blocktime:     nTx.Blocktime,
		BlockHeight:   nTx.BlockHeight,
	}

	return rt, nil
}

func (n NodeClient) GetTXOut(ctx context.Context, id string, outputIndex int, includeMempool bool) (res *bitcoin.TXOut, err error) {
	_, span := tracing.StartTracing(ctx, "NodeClient_GetRawTransaction", n.tracingEnabled, n.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	nTx, err := n.bitcoinClient.GetTxOut(id, outputIndex, includeMempool)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetRawTransaction, err)
	}

	return nTx, nil
}
