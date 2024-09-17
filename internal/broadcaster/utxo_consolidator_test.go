package broadcaster_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

func TestStart(t *testing.T) {
	ks, err := keyset.New()
	require.NoError(t, err)

	utxo1 := &sdkTx.UTXO{
		TxID:          testdata.TX1Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo2 := &sdkTx.UTXO{
		TxID:          testdata.TX2Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo3 := &sdkTx.UTXO{
		TxID:          testdata.TX3Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo4 := &sdkTx.UTXO{
		TxID:          testdata.TX4Hash[:],
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	tt := []struct {
		name                     string
		getUTXOsResp             sdkTx.UTXOs
		getUTXOsWithRetriesErr   error
		broadcastTransactionsErr error
		responseStatus           metamorph_api.Status

		expectedBroadcastTransactionsCalls int
		expectedError                      error
	}{
		{
			name:           "success",
			responseStatus: metamorph_api.Status_SEEN_ON_NETWORK,
			getUTXOsResp:   sdkTx.UTXOs{utxo1, utxo2, utxo3, utxo4},

			expectedBroadcastTransactionsCalls: 2,
		},
		{
			name:           "success - already consolidated",
			responseStatus: metamorph_api.Status_SEEN_ON_NETWORK,
			getUTXOsResp:   sdkTx.UTXOs{utxo1},

			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:                   "error - failed to get balance",
			getUTXOsWithRetriesErr: errors.New("utxo client error"),

			expectedError:                      broadcaster.ErrFailedToGetUTXOs,
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:           "consolidation not successful - status rejected",
			responseStatus: metamorph_api.Status_REJECTED,
			getUTXOsResp:   sdkTx.UTXOs{utxo1, utxo2, utxo3, utxo4},

			expectedBroadcastTransactionsCalls: 1,
		},
		{
			name:                     "error - broadcast transactions",
			broadcastTransactionsErr: errors.New("arc client error"),
			getUTXOsResp:             sdkTx.UTXOs{utxo1, utxo2, utxo3, utxo4},

			expectedBroadcastTransactionsCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			utxoClient := &mocks.UtxoClientMock{
				GetUTXOsWithRetriesFunc: func(ctx context.Context, lockingScript *script.Script, address string, constantBackoff time.Duration, retries uint64) (sdkTx.UTXOs, error) {
					return tc.getUTXOsResp, tc.getUTXOsWithRetriesErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			client := &mocks.ArcClientMock{
				BroadcastTransactionsFunc: func(ctx context.Context, txs sdkTx.Transactions, waitForStatus metamorph_api.Status, callbackURL string, callbackToken string, fullStatusUpdates bool, skipFeeValidation bool) ([]*metamorph_api.TransactionStatus, error) {
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
			sut, err := broadcaster.NewUTXOConsolidator(logger, client, ks, utxoClient, false,
				broadcaster.WithBatchSize(2),
				broadcaster.WithMaxInputs(2),
			)
			require.NoError(t, err)

			defer sut.Shutdown()

			// when
			actualError := sut.Start(1200)
			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				return
			} else {
				require.NoError(t, actualError)
			}

			time.Sleep(500 * time.Millisecond)

			// then
			require.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
		})
	}
}
