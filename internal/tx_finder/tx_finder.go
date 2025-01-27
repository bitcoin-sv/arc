package txfinder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/node_client"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

var (
	ErrFailedToGetMempoolAncestors = errors.New("failed to get mempool ancestors from node client")
)

type Finder struct {
	transactionHandler metamorph.TransactionHandler
	nodeClient         node_client.NodeClientI
	wocClient          *woc_client.WocClient
	logger             *slog.Logger
	tracingEnabled     bool
	tracingAttributes  []attribute.KeyValue
}

func WithTracerFinder(attr ...attribute.KeyValue) func(s *Finder) {
	return func(p *Finder) {
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

func New(th metamorph.TransactionHandler, n node_client.NodeClientI, w *woc_client.WocClient, l *slog.Logger, opts ...func(f *Finder)) *Finder {
	l = l.With(slog.String("module", "tx-finder"))

	f := &Finder{
		transactionHandler: th,
		nodeClient:         n,
		wocClient:          w,
		logger:             l,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

func (f Finder) GetMempoolAncestors(ctx context.Context, ids []string) ([]string, error) {
	txIDs, err := f.nodeClient.GetMempoolAncestors(ctx, ids)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetMempoolAncestors, err)
	}

	return txIDs, nil
}

func (f Finder) GetRawTxs(ctx context.Context, source validator.FindSourceFlag, ids []string) (txs []*sdkTx.Transaction, err error) {
	ctx, span := tracing.StartTracing(ctx, "Finder_GetRawTxs", f.tracingEnabled, f.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// NOTE: we can ignore ALL errors from providers, if one returns err we go to another
	foundTxs := make([]*sdkTx.Transaction, 0, len(ids))
	remainingIDs := map[string]struct{}{}

	for _, id := range ids {
		remainingIDs[id] = struct{}{}
	}

	// first get transactions from the handler
	if source.Has(validator.SourceTransactionHandler) {
		var thTxs []*metamorph.Transaction
		var thGetRawTxSpan trace.Span
		ctx, thGetRawTxSpan = tracing.StartTracing(ctx, "TransactionHandler_GetRawTxs", f.tracingEnabled, f.tracingAttributes...)
		thTxs, err = f.transactionHandler.GetTransactions(ctx, ids)
		tracing.EndTracing(thGetRawTxSpan, err)
		if err != nil {
			f.logger.WarnContext(ctx, "failed to get transactions from TransactionHandler", slog.Any("ids", ids), slog.String("err", err.Error()))
		} else {
			for _, thTx := range thTxs {
				var rt *sdkTx.Transaction
				rt, err = sdkTx.NewTransactionFromBytes(thTx.Bytes)
				if err != nil {
					f.logger.Error("failed to parse TransactionHandler tx bytes to transaction", slog.Any("id", thTx.TxID), slog.String("err", err.Error()))
					continue
				}

				delete(remainingIDs, thTx.TxID)
				foundTxs = append(foundTxs, rt)
			}
		}
	}

	ids = getKeys(remainingIDs)

	// try to get remaining txs from the node
	if source.Has(validator.SourceNodes) && f.nodeClient != nil {
		for _, id := range ids {
			rawTx, err := f.nodeClient.GetRawTransaction(ctx, id)
			if err != nil {
				f.logger.WarnContext(ctx, "failed to get transactions from bitcoin client", slog.String("id", id), slog.Any("err", err))
				continue
			}

			delete(remainingIDs, id)
			foundTxs = append(foundTxs, rawTx)
		}
	}

	ids = getKeys(remainingIDs)

	// at last try the WoC
	if source.Has(validator.SourceWoC) && len(ids) > 0 {
		var wocSpan trace.Span
		ctx, wocSpan = tracing.StartTracing(ctx, "WocClient_GetRawTxs", f.tracingEnabled, f.tracingAttributes...)
		var wocTxs []*woc_client.WocRawTx
		wocTxs, err = f.wocClient.GetRawTxs(ctx, ids)
		tracing.EndTracing(wocSpan, err)
		if err != nil {
			f.logger.WarnContext(ctx, "failed to get transactions from WoC", slog.Any("ids", ids), slog.Any("err", err))
		} else {
			for _, wTx := range wocTxs {
				if wTx.Error != "" {
					f.logger.WarnContext(ctx, "WoC tx reports error", slog.Any("id", ids), slog.Any("err", wTx.Error))
					continue
				}
				var tx *sdkTx.Transaction
				tx, err = sdkTx.NewTransactionFromHex(wTx.Hex)
				if err != nil {
					return nil, fmt.Errorf("failed to parse WoC hex string to transaction: %v", err)
				}

				foundTxs = append(foundTxs, tx)
			}
		}
	}

	return foundTxs, nil
}

func getKeys(uniqueMap map[string]struct{}) []string {
	uniqueKeys := make([]string, len(uniqueMap))
	counter := 0
	for id := range uniqueMap {
		uniqueKeys[counter] = id
		counter++
	}

	return uniqueKeys
}
