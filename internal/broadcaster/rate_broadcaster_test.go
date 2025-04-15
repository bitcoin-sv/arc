package broadcaster_test

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	chaincfg "github.com/bsv-blockchain/go-sdk/transaction/chaincfg"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

func TestRateBroadcasterStart(t *testing.T) {
	ks, err := keyset.New(&chaincfg.MainNet)
	require.NoError(t, err)

	tt := []struct {
		name                     string
		getBalanceWithRetriesErr error
		getUTXOsWithRetriesErr   error
		broadcastTransactionsErr error
		limit                    int64
		initialUtxoSetLen        int
		asyncTest                bool
		rateTxsPerSecond         int
		batchSize                int
		waitingTime              time.Duration
		transactionCount         int64
		jitterSize               int64

		expectedBroadcastTransactionsCalls int
		expectedError                      error
		expectedLimit                      int64
		expectedUtxoSetLen                 int
	}{
		{
			name:              "success",
			initialUtxoSetLen: 2,
			rateTxsPerSecond:  2,
			batchSize:         2,
			waitingTime:       1 * time.Millisecond,

			expectedBroadcastTransactionsCalls: 0,
			expectedUtxoSetLen:                 2,
		},
		{
			name:                     "error - failed to get balance",
			getBalanceWithRetriesErr: errors.New("utxo client error"),
			rateTxsPerSecond:         2,
			batchSize:                2,
			waitingTime:              1 * time.Millisecond,

			expectedError:                      broadcaster.ErrFailedToGetBalance,
			expectedBroadcastTransactionsCalls: 0,
			expectedUtxoSetLen:                 2,
		},
		{
			name:                   "error - failed to get utxos",
			getUTXOsWithRetriesErr: errors.New("failed to get utxos"),
			rateTxsPerSecond:       2,
			batchSize:              2,
			waitingTime:            1 * time.Millisecond,

			expectedError:                      broadcaster.ErrFailedToGetUTXOs,
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:              "broadcast transactions",
			initialUtxoSetLen: 2,
			rateTxsPerSecond:  2,
			batchSize:         2,
			waitingTime:       1 * time.Millisecond,

			expectedBroadcastTransactionsCalls: 0,
			expectedUtxoSetLen:                 2,
		},
		{
			name:              "success - limit reached",
			limit:             2,
			initialUtxoSetLen: 2,
			rateTxsPerSecond:  2,
			batchSize:         2,
			waitingTime:       1 * time.Millisecond,

			expectedBroadcastTransactionsCalls: 0,
			expectedLimit:                      2,
			expectedUtxoSetLen:                 2,
		},
		{
			name:              "error - utxo set smaller than batchSize",
			limit:             2,
			initialUtxoSetLen: 2,
			rateTxsPerSecond:  2,
			batchSize:         1000,
			waitingTime:       1 * time.Millisecond,

			expectedError:                      broadcaster.ErrTooSmallUTXOSet,
			expectedBroadcastTransactionsCalls: 0,
			expectedLimit:                      2,
		},
		{
			name:              "success - async batch with jitter",
			limit:             2,
			initialUtxoSetLen: 10,
			rateTxsPerSecond:  10,
			batchSize:         10,
			waitingTime:       2 * time.Second,
			transactionCount:  0,
			asyncTest:         true,

			expectedBroadcastTransactionsCalls: 0,
			expectedLimit:                      2,
			expectedUtxoSetLen:                 0,
			jitterSize:                         2,
		},
	}

	txIDbytes, _ := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	hash1, err := chainhash.NewHash(txIDbytes)
	require.NoError(t, err)
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			utxoClient := &mocks.UtxoClientMock{
				GetBalanceWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (uint64, uint64, error) {
					return 1000, 0, tc.getBalanceWithRetriesErr
				},
				GetUTXOsWithRetriesFunc: func(_ context.Context, lockingScript *script.Script, _ string, _ time.Duration, _ uint64) (sdkTx.UTXOs, error) {
					if tc.getUTXOsWithRetriesErr != nil {
						return nil, tc.getUTXOsWithRetriesErr
					}

					utxosToReturn := sdkTx.UTXOs{}
					baseUtxo := sdkTx.UTXOs{
						{
							TxID:          hash1,
							Vout:          0,
							LockingScript: lockingScript,
							Satoshis:      1000,
						},
					}
					for i := 0; i < tc.initialUtxoSetLen; i++ {
						utxosToReturn = append(utxosToReturn, baseUtxo...)
					}

					return utxosToReturn, nil
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

			tickerCh := make(chan time.Time, 5)
			ticker := &mocks.TickerMock{
				GetTickerChFunc: func() (<-chan time.Time, error) {
					return tickerCh, nil
				},
			}

			sut, err := broadcaster.NewRateBroadcaster(logger,
				client,
				ks,
				utxoClient,
				tc.limit,
				ticker,
				broadcaster.WithBatchSize(tc.batchSize),
				broadcaster.WithSizeJitter(tc.jitterSize),
				broadcaster.WithIsTestnet(false),
				broadcaster.WithCallback("callbackurl", "callbacktoken"),
				broadcaster.WithOpReturn("0"),
				broadcaster.WithFullstatusUpdates(false),
				broadcaster.WithFees(1),
				broadcaster.WithWaitForStatus(metamorph_api.Status_SEEN_ON_NETWORK),
				broadcaster.WithIsTestnet(true),
				broadcaster.WithMaxInputs(1),
			)
			require.NoError(t, err)

			// when
			err = sut.Start()
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			if tc.asyncTest {
				go func() {
					time.Sleep(100 * time.Millisecond)
					tickerCh <- time.Now()
				}()
			}

			time.Sleep(tc.waitingTime)

			// then
			require.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
			require.Equal(t, tc.expectedLimit, sut.GetLimit())
			require.Equal(t, int64(0), sut.GetConnectionCount())
			require.Equal(t, tc.transactionCount, sut.GetTxCount())
			require.Equal(t, tc.expectedUtxoSetLen, sut.GetUtxoSetLen())

			sut.Shutdown()

			sut.Wait()
		})
	}
}
