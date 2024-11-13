package txfinder

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
)

const (
	cacheExpiration = 2 * time.Second
	cacheCleanup    = 5 * time.Second
)

type CachedFinder struct {
	finder            *Finder
	cacheStore        *cache.Cache
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

func WithTracerCachedFinder(attr ...attribute.KeyValue) func(s *CachedFinder) {
	return func(p *CachedFinder) {
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

func WithCacheStore(store *cache.Cache) func(s *CachedFinder) {
	return func(p *CachedFinder) {
		p.cacheStore = store
	}
}

func NewCached(th metamorph.TransactionHandler, n NodeClient, w *woc_client.WocClient, l *slog.Logger, opts ...func(f *CachedFinder)) CachedFinder {
	c := CachedFinder{
		cacheStore: cache.New(cacheExpiration, cacheCleanup),
	}

	for _, opt := range opts {
		opt(&c)
	}
	var finderOpts []func(f *Finder)
	if c.tracingEnabled {
		finderOpts = append(finderOpts, WithTracerFinder(c.tracingAttributes...))
	}

	c.finder = New(th, n, w, l, finderOpts...)

	return c
}

func (f CachedFinder) GetMempoolAncestors(ctx context.Context, ids []string) ([]string, error) {
	return f.finder.GetMempoolAncestors(ctx, ids)
}

func (f CachedFinder) GetRawTxs(ctx context.Context, source validator.FindSourceFlag, ids []string) ([]*sdkTx.Transaction, error) {
	ctx, span := tracing.StartTracing(ctx, "CachedFinder_GetRawTxs", f.tracingEnabled, f.tracingAttributes...)
	defer tracing.EndTracing(span)

	cachedTxs := make([]*sdkTx.Transaction, 0, len(ids))
	var toFindIDs []string

	// check cache
	for _, id := range ids {
		value, found := f.cacheStore.Get(id)
		if found {
			cahchedTx, ok := value.(sdkTx.Transaction)
			if ok {
				cachedTxs = append(cachedTxs, &cahchedTx)
			}
		} else {
			toFindIDs = append(toFindIDs, id)
		}
	}

	if len(toFindIDs) == 0 {
		return cachedTxs, nil
	}

	// find txs
	foundTxs, err := f.finder.GetRawTxs(ctx, source, toFindIDs)
	if err != nil {
		return nil, err
	}

	// update cache
	for _, tx := range foundTxs {
		f.cacheStore.Set(tx.TxID(), *tx, cacheExpiration)
	}

	return append(cachedTxs, foundTxs...), nil
}
