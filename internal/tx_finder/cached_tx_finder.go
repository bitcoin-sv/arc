package txfinder

import (
	"context"
	"runtime"
	"time"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/tracing"
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

func NewCached(finder *Finder, opts ...func(f *CachedFinder)) CachedFinder {
	c := CachedFinder{
		cacheStore: cache.New(cacheExpiration, cacheCleanup),
		finder:     finder,
	}

	for _, opt := range opts {
		opt(&c)
	}

	return c
}

func (f CachedFinder) GetMempoolAncestors(ctx context.Context, ids []string) ([]string, error) {
	return f.finder.GetMempoolAncestors(ctx, ids)
}

func (f CachedFinder) GetRawTxs(ctx context.Context, source validator.FindSourceFlag, ids []string) (txs []*sdkTx.Transaction) {
	ctx, span := tracing.StartTracing(ctx, "CachedFinder_GetRawTxs", f.tracingEnabled, f.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, nil)
	}()

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
		return cachedTxs
	}

	// find txs
	foundTxs := f.finder.GetRawTxs(ctx, source, toFindIDs)

	// update cache
	for _, tx := range foundTxs {
		f.cacheStore.Set(tx.TxID().String(), *tx, cacheExpiration)
	}

	return append(cachedTxs, foundTxs...)
}
