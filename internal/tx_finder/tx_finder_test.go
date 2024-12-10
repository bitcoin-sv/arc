package txfinder

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/bitcoin-sv/arc/pkg/woc_client"

	transactionHandlerMock "github.com/bitcoin-sv/arc/pkg/metamorph/mocks"
)

func Test_GetRawTxs(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test")
	}

	tcs := []struct {
		name   string
		source validator.FindSourceFlag
		ids    []string

		thResponse func() ([]*metamorph.Transaction, error)
	}{
		{
			name:   "search Transaction Handler only - success",
			source: validator.SourceTransactionHandler,
			ids:    []string{"1", "2"},
			thResponse: func() ([]*metamorph.Transaction, error) {
				return []*metamorph.Transaction{
					{
						TxID: "1",
					},
					{
						TxID: "2",
					},
				}, nil
			},
		},
		{
			name:   "search Transaction Handler only - not found",
			source: validator.SourceTransactionHandler,
			ids:    []string{"1", "2"},
			thResponse: func() ([]*metamorph.Transaction, error) {
				return []*metamorph.Transaction{}, nil
			},
		},

		{
			name:   "search WoC only - success",
			source: validator.SourceWoC,
			ids: []string{
				"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df",
				"8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8",
			},
		},
		{
			name:   "search WoC only - not found",
			source: validator.SourceWoC,
			ids:    []string{"1", "2"},
		},

		{
			name:   "search Transacion Handler & WoC - find few in every source - success",
			source: validator.SourceTransactionHandler | validator.SourceWoC,
			ids: []string{
				"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df",
				"8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8",
			},
			thResponse: func() ([]*metamorph.Transaction, error) {
				return []*metamorph.Transaction{
					{
						TxID: "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8",
					},
				}, nil
			},
		},
		{
			name:   "search Transacion Handler & WoC - find few in every source - not found",
			source: validator.SourceTransactionHandler | validator.SourceWoC,
			ids: []string{
				"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df",
				"8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8",
				"1",
			},
			thResponse: func() ([]*metamorph.Transaction, error) {
				return []*metamorph.Transaction{
					{
						TxID: "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8",
					},
				}, nil
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// when
			var th metamorph.TransactionHandler
			if tc.source.Has(validator.SourceTransactionHandler) {
				th = transactionHandler(t, tc.thResponse)
			}

			var w *woc_client.WocClient
			if tc.source.Has(validator.SourceWoC) {
				w = woc_client.New(true)
			}

			l := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			sut := New(th, nil, w, l)

			// then

			_, err := sut.GetRawTxs(context.TODO(), tc.source, tc.ids)

			// assert
			require.NoError(t, err)
		})
	}
}

func transactionHandler(_ *testing.T, thResponse func() ([]*metamorph.Transaction, error)) metamorph.TransactionHandler {
	mq := transactionHandlerMock.TransactionHandlerMock{}
	mq.GetTransactionsFunc = func(_ context.Context, _ []string) ([]*metamorph.Transaction, error) {
		return thResponse()
	}

	return &mq
}
