package node_client

import (
	"context"
	"errors"
	"runtime"
	"strings"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/pkg/rpc_client"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

var (
	ErrFailedToGetRawTransaction   = errors.New("failed to get raw transaction")
	ErrFailedToGetMempoolAncestors = errors.New("failed to get mempool ancestors")
	ErrFailedToSendRawTransaction  = errors.New("failed to send raw transaction")
)

type NodeClient struct {
	bitcoinClient     *rpc_client.RPCClient
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
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

func New(n *rpc_client.RPCClient, opts ...func(client *NodeClient)) (NodeClient, error) {
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
		txIDs, err := n.bitcoinClient.GetMempoolAncestors(ctx, id)
		tracing.EndTracing(getMemPoolAncSpan, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Transaction not in mempool") {
				continue
			}

			return nil, errors.Join(ErrFailedToGetMempoolAncestors, err)
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

	nTx, err := n.bitcoinClient.GetRawTransactionHex(ctx, id)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetRawTransaction, err)
	}

	rt, err = sdkTx.NewTransactionFromHex(nTx)
	if err != nil {
		return nil, err
	}

	return rt, nil
}
