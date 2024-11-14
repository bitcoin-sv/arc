package txfinder

import (
	"context"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/bitcoin-sv/arc/pkg/metamorph/mocks"
)

// Mocked data for RawTx
var rawTx1 = validator.RawTx{TxID: "tx1"}
var rawTx2 = validator.RawTx{TxID: "tx2"}

func TestCachedFinder_GetRawTxs_AllFromCache(t *testing.T) {
	tt := []struct {
		name      string
		cachedTx  []validator.RawTx
		fetchedTx []*metamorph.Transaction
	}{
		{
			name:     "all from cache",
			cachedTx: []validator.RawTx{rawTx1, rawTx2},
		},
		{
			name:      "all from finder",
			fetchedTx: []*metamorph.Transaction{{TxID: rawTx1.TxID}, {TxID: rawTx2.TxID}},
		},
		{
			name:      "cached and fetched mixed",
			cachedTx:  []validator.RawTx{rawTx1},
			fetchedTx: []*metamorph.Transaction{{TxID: rawTx2.TxID}},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			thMq := &mocks.TransactionHandlerMock{
				GetTransactionsFunc: func(_ context.Context, _ []string) ([]*metamorph.Transaction, error) {
					return tc.fetchedTx, nil
				},
			}

			c := cache.New(10*time.Second, 10*time.Second)
			for _, r := range tc.cachedTx {
				c.Set(r.TxID, r, cache.DefaultExpiration)
			}

			sut := CachedFinder{
				finder: &Finder{
					transactionHandler: thMq,
				},
				cacheStore: c,
			}

			// when
			// try to find in cache or with TransactionHandler only
			res, err := sut.GetRawTxs(context.Background(), validator.SourceTransactionHandler, []string{rawTx1.TxID, rawTx2.TxID})

			// then
			require.NoError(t, err)
			require.Len(t, res, len(tc.cachedTx)+len(tc.fetchedTx))

			if len(tc.fetchedTx) > 0 {
				require.Len(t, thMq.GetTransactionsCalls(), 1)
			} else {
				require.Len(t, thMq.GetTransactionsCalls(), 0, "Transaction handler shoulnd not be called when all transactions were already in cache")
			}
		})
	}
}
