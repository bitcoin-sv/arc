package txfinder

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/global/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/internal/validator"
)

func TestCachedFinder_GetRawTxs_AllFromCache(t *testing.T) {
	tt := []struct {
		name      string
		cachedTx  []sdkTx.Transaction
		fetchedTx []*global.Transaction
	}{
		{
			name:     "all from cache",
			cachedTx: []sdkTx.Transaction{*testdata.TX1Raw, *testdata.TX6Raw},
		},
		{
			name: "all from finder",
			fetchedTx: []*global.Transaction{
				{TxID: testdata.TX1Raw.TxID().String(), Bytes: testdata.TX1Raw.Bytes()},
				{TxID: testdata.TX6Raw.TxID().String(), Bytes: testdata.TX6Raw.Bytes()},
			},
		},
		{
			name:     "cached and fetched mixed",
			cachedTx: []sdkTx.Transaction{*testdata.TX1Raw},
			fetchedTx: []*global.Transaction{
				{TxID: testdata.TX6Raw.TxID().String(), Bytes: testdata.TX6Raw.Bytes()},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			thMq := &mocks.TransactionHandlerMock{
				GetTransactionsFunc: func(_ context.Context, _ []string) ([]*global.Transaction, error) {
					return tc.fetchedTx, nil
				},
			}

			c := cache.New(10*time.Second, 10*time.Second)
			for _, r := range tc.cachedTx {
				c.Set(r.TxID().String(), r, cache.DefaultExpiration)
			}
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			finder := New(thMq, nil, nil, logger)
			sut := NewCached(finder, WithCacheStore(c))

			// when
			// try to find in cache or with TransactionHandler only
			res := sut.GetRawTxs(context.Background(), validator.SourceTransactionHandler, []string{testdata.TX1Raw.TxID().String(), testdata.TX6Raw.TxID().String()})

			// then
			require.Len(t, res, len(tc.cachedTx)+len(tc.fetchedTx))

			if len(tc.fetchedTx) > 0 {
				require.Len(t, thMq.GetTransactionsCalls(), 1)
			} else {
				require.Len(t, thMq.GetTransactionsCalls(), 0, "Transaction handler shoulnd not be called when all transactions were already in cache")
			}
		})
	}
}
