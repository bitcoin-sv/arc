package broadcaster_test

import (
	"context"
	"errors"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	ks, err := keyset.New()
	require.NoError(t, err)

	utxo1 := &transaction.UTXO{
		TxID:          testdata.TX1Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo2 := &transaction.UTXO{
		TxID:          testdata.TX2Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo3 := &transaction.UTXO{
		TxID:          testdata.TX3Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo4 := &transaction.UTXO{
		TxID:          testdata.TX4Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	tt := []struct {
		name                     string
		getBalanceWithRetriesErr error
		getUTXOsWithRetriesErr   error
		broadcastTransactionsErr error
		responseStatus           metamorph_api.Status

		expectedBroadcastTransactionsCalls int
		expectedErrorStr                   string
	}{
		{
			name:           "success",
			responseStatus: metamorph_api.Status_SEEN_ON_NETWORK,

			expectedBroadcastTransactionsCalls: 2,
		},
		{
			name:                     "error - failed to get balance",
			getBalanceWithRetriesErr: errors.New("utxo client error"),

			expectedErrorStr:                   "failed to get balance",
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:                   "error - failed to get balance",
			getUTXOsWithRetriesErr: errors.New("utxo client error"),

			expectedErrorStr:                   "failed to get utxos",
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:           "consolidation not successful - status rejected",
			responseStatus: metamorph_api.Status_REJECTED,

			expectedBroadcastTransactionsCalls: 1,
		},
		{
			name:                     "error - broadcast transactions",
			broadcastTransactionsErr: errors.New("arc client error"),

			expectedBroadcastTransactionsCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			utxoClient := &mocks.UtxoClientMock{
				GetBalanceWithRetriesFunc: func(ctx context.Context, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error) {
					return 1000, 0, tc.getBalanceWithRetriesErr
				},
				GetUTXOsWithRetriesFunc: func(ctx context.Context, lockingScript *script.Script, address string, constantBackoff time.Duration, retries uint64) (transaction.UTXOs, error) {
					utxos := transaction.UTXOs{utxo1, utxo2, utxo3, utxo4}

					return utxos, tc.getUTXOsWithRetriesErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			client := &mocks.ArcClientMock{
				BroadcastTransactionsFunc: func(ctx context.Context, txs transaction.Transactions, waitForStatus metamorph_api.Status, callbackURL string, callbackToken string, fullStatusUpdates bool, skipFeeValidation bool) ([]*metamorph_api.TransactionStatus, error) {
					var statuses []*metamorph_api.TransactionStatus

					for _, tx := range txs {
						statuses = append(statuses, &metamorph_api.TransactionStatus{
							Txid:   tx.TxID(),
							Status: tc.responseStatus,
						})
					}

					return statuses, tc.broadcastTransactionsErr
				},
			}
			c, err := broadcaster.NewUTXOConsolidator(logger, client, ks, utxoClient, false,
				broadcaster.WithBatchSize(2),
				broadcaster.WithMaxInputs(2),
			)

			require.NoError(t, err)

			err = c.Start()
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			time.Sleep(500 * time.Millisecond)

			c.Shutdown()

			require.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
		})
	}
}
