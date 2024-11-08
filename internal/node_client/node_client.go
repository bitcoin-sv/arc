package node_client

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/validator"
)

type NodeClient struct {
	bitcoinClient     *bitcoin.Bitcoind
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

func New(opts ...func(client *NodeClient)) NodeClient {
	n := NodeClient{}

	for _, opt := range opts {
		opt(&n)
	}

	return n
}

func (f NodeClient) GetMempoolAncestors(ctx context.Context, ids []string) ([]validator.RawTx, error) {
	rawTxs := make([]validator.RawTx, 0, len(ids))
	for _, id := range ids {
		_, span := tracing.StartTracing(ctx, "Bitcoind_GetMempoolAncestors", f.tracingEnabled, f.tracingAttributes...)
		nTx, err := f.bitcoinClient.GetMempoolAncestors(id, false)
		tracing.EndTracing(span)
		if err != nil {
			return nil, fmt.Errorf("failed to get mempool ancestors: %v", err)
		}

		if nTx == nil {
			return nil, nil
		}

		var rawTx *bitcoin.RawTransaction

		err = json.Unmarshal(nTx, &rawTx)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw transaction: %v", err)
		}

		rt, err := validator.NewRawTx(rawTx.TxID, rawTx.Hex, rawTx.BlockHeight)
		if err != nil {
			return nil, err
		}

		rawTxs = append(rawTxs, rt)
	}

	return rawTxs, nil
}

func (f NodeClient) GetRawTransaction(ctx context.Context, id string) (validator.RawTx, error) {
	_, span := tracing.StartTracing(ctx, "Bitcoind_GetRawTransaction", f.tracingEnabled, f.tracingAttributes...)
	nTx, err := f.bitcoinClient.GetRawTransaction(id)
	tracing.EndTracing(span)
	if err != nil {
		return validator.RawTx{}, fmt.Errorf("failed to get raw transaction: %v", err)
	}

	rt, err := validator.NewRawTx(nTx.TxID, nTx.Hex, nTx.BlockHeight)
	if err != nil {
		return validator.RawTx{}, err
	}

	return rt, nil
}
