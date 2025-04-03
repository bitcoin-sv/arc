package broadcaster_test

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
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
		expectedLimit                      int64
		initialUtxoSetLen                  int
		skipExpectedUtxoSetLen             bool
		rateTxsPerSecond                   int
		batchSize                          int
		waitingTime                        time.Duration
		transactionCount                   int64
	}{
		{
			name:                               "success",
			expectedBroadcastTransactionsCalls: 0,
			initialUtxoSetLen:                  2,
			rateTxsPerSecond:                   2,
			batchSize:                          2,
			waitingTime:                        1 * time.Millisecond,
		},
		{
			name:                               "error - failed to get balance",
			getBalanceWithRetriesErr:           errors.New("utxo client error"),
			expectedError:                      broadcaster.ErrFailedToGetBalance,
			expectedBroadcastTransactionsCalls: 0,
			rateTxsPerSecond:                   2,
			batchSize:                          2,
			waitingTime:                        1 * time.Millisecond,
		},
		{
			name:                               "error - failed to get utxos",
			getUTXOsWithRetriesErr:             errors.New("failed to get utxos"),
			expectedError:                      broadcaster.ErrFailedToGetUTXOs,
			expectedBroadcastTransactionsCalls: 0,
			rateTxsPerSecond:                   2,
			batchSize:                          2,
			waitingTime:                        1 * time.Millisecond,
		},
		{
			name:                               "broadcast transactions",
			expectedBroadcastTransactionsCalls: 0,
			initialUtxoSetLen:                  2,
			rateTxsPerSecond:                   2,
			batchSize:                          2,
			waitingTime:                        1 * time.Millisecond,
		},
		{
			name:                               "success - limit reached",
			limit:                              2,
			expectedBroadcastTransactionsCalls: 0,
			expectedLimit:                      2,
			initialUtxoSetLen:                  2,
			rateTxsPerSecond:                   2,
			batchSize:                          2,
			waitingTime:                        1 * time.Millisecond,
		},
		{
			name:                               "error - too high rateTxPerSecond",
			limit:                              2,
			expectedBroadcastTransactionsCalls: 0,
			expectedLimit:                      2,
			initialUtxoSetLen:                  2,
			rateTxsPerSecond:                   1500,
			expectedError:                      broadcaster.ErrTooHighSubmissionRate,
			batchSize:                          1,
			waitingTime:                        1 * time.Millisecond,
		},
		{
			name:                               "error - utxo set smaller than batchSize",
			limit:                              2,
			expectedBroadcastTransactionsCalls: 0,
			expectedLimit:                      2,
			initialUtxoSetLen:                  2,
			rateTxsPerSecond:                   2,
			expectedError:                      broadcaster.ErrTooSmallUTXOSet,
			batchSize:                          1000,
			waitingTime:                        1 * time.Millisecond,
		},
		{
			name:                               "success - asynch batch",
			limit:                              2,
			expectedBroadcastTransactionsCalls: 1,
			expectedLimit:                      2,
			initialUtxoSetLen:                  60,
			rateTxsPerSecond:                   10,
			batchSize:                          10,
			waitingTime:                        7 * time.Second,
			transactionCount:                   10,
			skipExpectedUtxoSetLen:             true,
		},
	}

	txIDbytes, _ := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	hash1, err := chainhash.NewHash(txIDbytes)
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

			sut, err := broadcaster.NewRateBroadcaster(logger, client, ks, utxoClient, false, tc.rateTxsPerSecond, tc.limit, broadcaster.WithBatchSize(tc.batchSize), broadcaster.WithSizeJitter(1))
			require.NoError(t, err)

			// when
			err = sut.Start()
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			if tc.waitingTime > time.Second {
				wg := sync.WaitGroup{}
				wg.Add(1)
				done := make(chan struct{})
				go func() {
					wg.Wait()
					close(done)
				}()
				select {
				case <-time.After(tc.waitingTime):
					t.Logf("waiting time over")
				case <-done:
				}
			}

			go sut.Wait()

			sut.Shutdown()

			// then
			require.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
			require.Equal(t, tc.transactionCount, sut.GetTxCount())
			require.Equal(t, tc.expectedLimit, sut.GetLimit())
			require.Equal(t, int64(0), sut.GetConnectionCount())
			if !tc.skipExpectedUtxoSetLen {
				require.Equal(t, tc.initialUtxoSetLen, sut.GetUtxoSetLen())
			}
		})
	}
}
