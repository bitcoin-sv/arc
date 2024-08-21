package broadcaster_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/stretchr/testify/require"
)

func TestMultiKeyRateBroadcaster_Start(t *testing.T) {
	ks1, err := keyset.New()
	require.NoError(t, err)

	ks2, err := keyset.New()
	require.NoError(t, err)

	utxo1 := &bt.UTXO{
		TxID:          testdata.TX1Hash[:],
		Vout:          0,
		LockingScript: ks1.Script,
		Satoshis:      1000,
	}
	utxo2 := &bt.UTXO{
		TxID:          testdata.TX2Hash[:],
		Vout:          0,
		LockingScript: ks2.Script,
		Satoshis:      1000,
	}

	tt := []struct {
		name                     string
		getBalanceWithRetriesErr error
		getUTXOsWithRetriesErr   error

		expectedBroadcastTransactionsCalls int
		expectedErrorStr                   string
	}{
		{
			name: "success",

			expectedBroadcastTransactionsCalls: 2,
		},
		{
			name:                     "error - failed to get balance",
			getBalanceWithRetriesErr: errors.New("utxo client error"),

			expectedErrorStr:                   "failed to get balance",
			expectedBroadcastTransactionsCalls: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			utxoClient := &mocks.UtxoClientMock{
				GetBalanceWithRetriesFunc: func(ctx context.Context, mainnet bool, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error) {
					return 1000, 0, tc.getBalanceWithRetriesErr
				},
				GetUTXOsWithRetriesFunc: func(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) ([]*bt.UTXO, error) {
					return []*bt.UTXO{utxo1, utxo2}, tc.getUTXOsWithRetriesErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			client := &mocks.ArcClientMock{
				BroadcastTransactionsFunc: func(ctx context.Context, txs []*bt.Tx, waitForStatus metamorph_api.Status, callbackURL string, callbackToken string, fullStatusUpdates bool, skipFeeValidation bool) ([]*metamorph_api.TransactionStatus, error) {
					var statuses []*metamorph_api.TransactionStatus

					for _, tx := range txs {
						statuses = append(statuses, &metamorph_api.TransactionStatus{
							Txid:   tx.TxID(),
							Status: metamorph_api.Status_SEEN_ON_NETWORK,
						})
					}

					return statuses, nil
				},
			}

			mrb, err := broadcaster.NewMultiKeyRateBroadcaster(logger, client, []*keyset.KeySet{ks1, ks2}, utxoClient, false, broadcaster.WithBatchSize(2))
			require.NoError(t, err)

			err = mrb.Start(10, 50)
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			time.Sleep(500 * time.Millisecond)

			mrb.Shutdown()
		})
	}
}
