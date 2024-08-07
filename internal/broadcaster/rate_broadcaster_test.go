package broadcaster_test

import (
	"context"
	"errors"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	ks, err := keyset.New()
	require.NoError(t, err)

	utxo1 := &bt.UTXO{
		TxID:          testdata.TX1Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo2 := &bt.UTXO{
		TxID:          testdata.TX2Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo3 := &bt.UTXO{
		TxID:          testdata.TX3Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo4 := &bt.UTXO{
		TxID:          testdata.TX4Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
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
		// Todo: "error - failed to get utxos",
		// Todo: "error - broadcast transactions"
		// Todo: "success - limit reached"
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			utxoClient := &mocks.UtxoClientMock{
				GetBalanceWithRetriesFunc: func(ctx context.Context, mainnet bool, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error) {
					return 1000, 0, tc.getBalanceWithRetriesErr
				},
				GetUTXOsWithRetriesFunc: func(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) ([]*bt.UTXO, error) {
					utxos := []*bt.UTXO{utxo1, utxo2, utxo3, utxo4}

					return utxos, tc.getUTXOsWithRetriesErr
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
			rb, err := broadcaster.NewRateBroadcaster(logger, client, ks, utxoClient, false, broadcaster.WithBatchSize(2))

			require.NoError(t, err)

			err = rb.Start(10, 50)
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			time.Sleep(500 * time.Millisecond)

			rb.Shutdown()

			require.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
		})
	}
}
