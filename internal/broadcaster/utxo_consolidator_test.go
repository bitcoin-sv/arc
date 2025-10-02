package broadcaster_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	chaincfg "github.com/bsv-blockchain/go-sdk/transaction/chaincfg"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

func TestStart(t *testing.T) {
	ks, err := keyset.New(&chaincfg.MainNet)
	require.NoError(t, err)

	hash1, _ := chainhash.NewHash(testdata.TX1Hash[:])
	hash2, _ := chainhash.NewHash(testdata.TX2Hash[:])
	hash3, _ := chainhash.NewHash(testdata.TX3Hash[:])
	hash4, _ := chainhash.NewHash(testdata.TX4Hash[:])

	utxo1 := &sdkTx.UTXO{
		TxID:          hash1,
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo2 := &sdkTx.UTXO{
		TxID:          hash2,
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo3 := &sdkTx.UTXO{
		TxID:          hash3,
		Vout:          0,
		LockingScript: ks.Script,
		Satoshis:      1000,
	}
	utxo4 := &sdkTx.UTXO{
		TxID:          hash4,
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
				GetUTXOsWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64, _ int) (sdkTx.UTXOs, error) {
					return tc.getUTXOsResp, tc.getUTXOsWithRetriesErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			client := &mocks.ArcClientMock{
				BroadcastTransactionsFunc: func(_ context.Context, txs sdkTx.Transactions, _ metamorph_api.Status, _ string, _ string, _ bool, _ bool) ([]*metamorph_api.TransactionStatus, error) {
					var statuses []*metamorph_api.TransactionStatus

					for _, tx := range txs {
						statuses = append(statuses, &metamorph_api.TransactionStatus{
							Txid:   tx.TxID().String(),
							Status: tc.responseStatus,
						})
					}

					return statuses, tc.broadcastTransactionsErr
				},
			}
			sut, err := broadcaster.NewUTXOConsolidator(
				logger,
				client,
				ks,
				utxoClient,
				broadcaster.WithBatchSize(2),
				broadcaster.WithMaxInputs(2),
				broadcaster.WithIsTestnet(false),
			)
			require.NoError(t, err)

			defer sut.Shutdown()

			// when
			actualError := sut.Start(1200)
			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				return
			}
			require.NoError(t, actualError)

			time.Sleep(500 * time.Millisecond)

			// then
			require.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
		})
	}
}
