package broadcaster_test

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	chaincfg "github.com/bsv-blockchain/go-sdk/transaction/chaincfg"
	"github.com/stretchr/testify/require"
)

func TestRateBroadcaster(t *testing.T) {
	ks, err := keyset.New(&chaincfg.MainNet)
	require.NoError(t, err)

	tt := []struct {
		name                               string
		getBalanceWithRetriesErr           error
		getUTXOsWithRetriesErr             error
		broadcastTransactionsErr           error
		limit                              int64
		expectedBroadcastTransactionsCalls int
		expectedError                      error
	}{
		{
			name:                               "success",
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:                               "error - failed to get balance",
			getBalanceWithRetriesErr:           errors.New("utxo client error"),
			expectedError:                      broadcaster.ErrFailedToGetBalance,
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:                               "error - failed to get utxos",
			getUTXOsWithRetriesErr:             errors.New("failed to get utxos"),
			expectedError:                      broadcaster.ErrFailedToGetUTXOs,
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:                               "broadcast transactions",
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:                               "success - limit reached",
			limit:                              2,
			expectedBroadcastTransactionsCalls: 0,
		},
	}

	txIDbytes, _ := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	hash1, err := chainhash.NewHash(txIDbytes)
	require.NoError(t, err)

	txIDbytes, _ = hex.DecodeString("1a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	hash2, err := chainhash.NewHash(txIDbytes)
	require.NoError(t, err)
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			utxoClient := &mocks.UtxoClientMock{
				GetBalanceWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (int64, int64, error) {
					return 1000, 0, tc.getBalanceWithRetriesErr
				},
				GetUTXOsWithRetriesFunc: func(_ context.Context, lockingScript *script.Script, _ string, _ time.Duration, _ uint64) (sdkTx.UTXOs, error) {
					if tc.getUTXOsWithRetriesErr != nil {
						return nil, tc.getUTXOsWithRetriesErr
					}

					return sdkTx.UTXOs{
						{
							TxID:          hash1,
							Vout:          0,
							LockingScript: lockingScript,
							Satoshis:      1000,
						},
						{
							TxID:          hash2,
							Vout:          1,
							LockingScript: lockingScript,
							Satoshis:      1000,
						},
					}, nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			client := &mocks.ArcClientMock{
				BroadcastTransactionsFunc: func(_ context.Context, txs sdkTx.Transactions, _ metamorph_api.Status, _ string, _ string, _ bool, _ bool) ([]*metamorph_api.TransactionStatus, error) {
					if tc.broadcastTransactionsErr != nil {
						return nil, tc.broadcastTransactionsErr
					}
					var statuses []*metamorph_api.TransactionStatus
					for _, tx := range txs {
						statuses = append(statuses, &metamorph_api.TransactionStatus{
							Txid:   tx.TxID().String(),
							Status: metamorph_api.Status_SEEN_ON_NETWORK,
						})
					}
					return statuses, nil
				},
			}

			sut, err := broadcaster.NewRateBroadcaster(logger, client, ks, utxoClient, false, 2, tc.limit, broadcaster.WithBatchSize(2))
			require.NoError(t, err)

			// when
			err = sut.Start()
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			time.Sleep(500 * time.Millisecond)

			sut.Shutdown()

			// then
			require.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
		})
	}
}
