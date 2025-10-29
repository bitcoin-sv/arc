package broadcaster_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	transaction "github.com/bsv-blockchain/go-sdk/transaction/chaincfg"
	"github.com/stretchr/testify/require"
)

func TestUTXOCreator(t *testing.T) {
	ks, err := keyset.New(&transaction.Params{})
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	hash1, _ := chainhash.NewHash(testdata.TX1Hash[:])

	tests := []struct {
		name                   string
		getUTXOsResp           sdkTx.UTXOs
		getBalanceFunc         func(ctx context.Context, address string, constantBackoff time.Duration, retries uint64) (uint64, uint64, error)
		expectedBroadcastCalls int
		expectedError          error
		requestedUTXOs         uint64
		requestedAmountPerUTXO uint64
	}{
		{
			name:         "success - creates correct UTXOs",
			getUTXOsResp: sdkTx.UTXOs{{TxID: hash1, Vout: 0, LockingScript: ks.Script, Satoshis: 401}},
			getBalanceFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (uint64, uint64, error) {
				return 400, 0, nil
			},
			expectedBroadcastCalls: 1,
			expectedError:          nil,
			requestedUTXOs:         4,
			requestedAmountPerUTXO: 100,
		},
		{
			name:         "Insufficient balance",
			getUTXOsResp: sdkTx.UTXOs{{TxID: hash1, Vout: 0, LockingScript: ks.Script, Satoshis: 50}},
			getBalanceFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (uint64, uint64, error) {
				return 100, 0, nil
			},
			expectedBroadcastCalls: 0,
			expectedError:          broadcaster.ErrRequestedSatoshisTooHigh,
			requestedUTXOs:         4,
			requestedAmountPerUTXO: 100,
		},
	}

	for _, tt := range tests {
		testutils.RunParallel(t, true, tt.name, func(t *testing.T) {
			// Given
			mockArcClient := &mocks.ArcClientMock{
				BroadcastTransactionsFunc: func(_ context.Context, txs sdkTx.Transactions, _ metamorph_api.Status, _, _ string, _, _ bool) ([]*metamorph_api.TransactionStatus, error) {
					statuses := make([]*metamorph_api.TransactionStatus, len(txs))
					for i, tx := range txs {
						statuses[i] = &metamorph_api.TransactionStatus{Txid: tx.TxID().String(), Status: metamorph_api.Status_SEEN_ON_NETWORK}
					}
					return statuses, nil
				},
			}

			mockUtxoClient := &mocks.UtxoClientMock{
				GetBalanceWithRetriesFunc: tt.getBalanceFunc,
				GetUTXOsWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64, _ int) (sdkTx.UTXOs, error) {
					return tt.getUTXOsResp, nil
				},
			}

			utxoCreator, err := broadcaster.NewUTXOCreator(logger, mockArcClient, ks, mockUtxoClient, broadcaster.WithIsTestnet(false))
			require.NoError(t, err)
			defer utxoCreator.Shutdown()

			// When
			actualError := utxoCreator.Start(tt.requestedUTXOs, tt.requestedAmountPerUTXO)

			// Then
			if tt.expectedError != nil {
				require.ErrorIs(t, actualError, tt.expectedError)
				return
			}

			require.NoError(t, actualError)

			time.Sleep(500 * time.Millisecond)

			require.Equal(t, tt.expectedBroadcastCalls, len(mockArcClient.BroadcastTransactionsCalls()), "Unexpected number of BroadcastTransactions calls")

			if tt.expectedBroadcastCalls > 0 {
				for _, call := range mockArcClient.BroadcastTransactionsCalls() {
					for _, tx := range call.Txs {
						for _, output := range tx.Outputs {
							require.Equal(t, tt.requestedAmountPerUTXO, output.Satoshis, "Each UTXO output should have the requested amount")
						}
					}
				}
			}
		})
	}
}
