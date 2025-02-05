package txfinder

import (
	"context"
	"errors"
	"log/slog"
	"runtime"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/tracing"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

var (
	ErrFailedToGetMempoolAncestors = errors.New("failed to get mempool ancestors from node client")
)

type Finder struct {
	transactionHandler metamorph.TransactionHandler
	bitcoinClient      NodeClient
	wocClient          WocClient
	logger             *slog.Logger
	tracingEnabled     bool
	tracingAttributes  []attribute.KeyValue
}

type WocClient interface {
	GetRawTxs(ctx context.Context, ids []string) (result []*woc_client.WocRawTx, err error)
}

type NodeClient interface {
	GetMempoolAncestors(ctx context.Context, ids []string) ([]string, error)
	GetRawTransaction(ctx context.Context, id string) (*sdkTx.Transaction, error)
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

func New(th metamorph.TransactionHandler, n NodeClient, w WocClient, l *slog.Logger, opts ...func(f *Finder)) *Finder {
	l = l.With(slog.String("module", "tx-finder"))

	f := &Finder{
		transactionHandler: th,
		bitcoinClient:      n,
		wocClient:          w,
		logger:             l,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

func (f Finder) GetMempoolAncestors(ctx context.Context, ids []string) ([]string, error) {
	txIDs, err := f.bitcoinClient.GetMempoolAncestors(ctx, ids)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetMempoolAncestors, err)
	}

	return txIDs, nil
}

func (f Finder) getRawTxsFromTransactionHandler(ctx context.Context, foundTxs *[]*sdkTx.Transaction, remainingIDs map[string]struct{}) {
	ctx, span := tracing.StartTracing(ctx, "Finder_getRawTxsFromTransactionHandler", f.tracingEnabled, f.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, nil)
	}()

	ids := getKeys(remainingIDs)
	var thTxs []*metamorph.Transaction
	var err error
	thTxs, err = f.transactionHandler.GetTransactions(ctx, ids)
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
			*foundTxs = append(*foundTxs, rt)
		}
	}
}

func (f Finder) getRawTxsFromNode(ctx context.Context, foundTxs *[]*sdkTx.Transaction, remainingIDs map[string]struct{}) {
	ctx, span := tracing.StartTracing(ctx, "Finder_getRawTxsFromNode", f.tracingEnabled, f.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, nil)
	}()

	ids := getKeys(remainingIDs)
	for _, id := range ids {
		rawTx, err := f.bitcoinClient.GetRawTransaction(ctx, id)
		if err != nil {
			f.logger.WarnContext(ctx, "failed to get transactions from bitcoin client", slog.String("id", id), slog.Any("err", err))
			continue
		}

		delete(remainingIDs, id)
		*foundTxs = append(*foundTxs, rawTx)
	}
}

func (f Finder) getRawTxsFromWoc(ctx context.Context, foundTxs *[]*sdkTx.Transaction, remainingIDs map[string]struct{}) {
	ctx, span := tracing.StartTracing(ctx, "Finder_getRawTxsFromWoc", f.tracingEnabled, f.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, nil)
	}()

	var wocTxs []*woc_client.WocRawTx
	var err error
	ids := getKeys(remainingIDs)
	wocTxs, err = f.wocClient.GetRawTxs(ctx, ids)
	if err != nil {
		f.logger.WarnContext(ctx, "failed to get transactions from WoC", slog.Any("err", err))
	} else {
		for _, wTx := range wocTxs {
			if wTx.Error != "" {
				f.logger.WarnContext(ctx, "WoC tx reports error", slog.Any("err", wTx.Error))
				continue
			}

			var tx *sdkTx.Transaction
			tx, err = sdkTx.NewTransactionFromHex(wTx.Hex)
			if err != nil {
				f.logger.WarnContext(ctx, "failed to parse WoC hex string to transaction", slog.Any("err", err.Error()))
				continue
			}

			*foundTxs = append(*foundTxs, tx)
		}
	}
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
	if len(remainingIDs) > 0 && source.Has(validator.SourceTransactionHandler) {
		f.getRawTxsFromTransactionHandler(ctx, &foundTxs, remainingIDs)
	}

	// try to get remaining txs from the node
	if len(remainingIDs) > 0 && source.Has(validator.SourceNodes) && f.bitcoinClient != nil {
		f.getRawTxsFromNode(ctx, &foundTxs, remainingIDs)
	}

	// at last try the WoC
	if len(remainingIDs) > 0 && source.Has(validator.SourceWoC) {
		f.getRawTxsFromWoc(ctx, &foundTxs, remainingIDs)
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
