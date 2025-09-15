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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

func TestRateBroadcasterInitialize(t *testing.T) {
	ks, err := keyset.New(&chaincfg.MainNet)
	require.NoError(t, err)

	tt := []struct {
		name                     string
		getBalanceWithRetriesErr error
		getUTXOsWithRetriesErr   error
		limit                    int64
		initialUtxoSetLen        int
		batchSize                int

		expectedBroadcastTransactionsCalls int
		expectedError                      error
		expectedLimit                      int64
		expectedUtxoSetLen                 int
	}{
		{
			name:              "success",
			initialUtxoSetLen: 2,
			batchSize:         2,

			expectedBroadcastTransactionsCalls: 0,
			expectedUtxoSetLen:                 2,
		},
		{
			name:                     "error - failed to get balance",
			getBalanceWithRetriesErr: errors.New("utxo client error"),
			batchSize:                2,

			expectedError:                      broadcaster.ErrFailedToGetBalance,
			expectedBroadcastTransactionsCalls: 0,
			expectedUtxoSetLen:                 2,
		},
		{
			name:                   "error - failed to get utxos",
			getUTXOsWithRetriesErr: errors.New("failed to get utxos"),
			batchSize:              2,

			expectedError:                      broadcaster.ErrFailedToGetUTXOs,
			expectedBroadcastTransactionsCalls: 0,
		},
		{
			name:              "success - limit reached",
			limit:             2,
			initialUtxoSetLen: 2,
			batchSize:         2,

			expectedBroadcastTransactionsCalls: 0,
			expectedLimit:                      2,
			expectedUtxoSetLen:                 2,
		},
		{
			name:              "error - utxo set smaller than batchSize",
			limit:             2,
			initialUtxoSetLen: 2,
			batchSize:         1000,

			expectedError:                      broadcaster.ErrTooSmallUTXOSet,
			expectedBroadcastTransactionsCalls: 0,
			expectedLimit:                      2,
		},
	}

	tickerCh := make(chan time.Time, 5)
	ticker := &mocks.TickerMock{
		GetTickerChFunc: func() <-chan time.Time {
			return tickerCh
		},
	}
	txIDBytes, _ := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	hash1, err := chainhash.NewHash(txIDBytes)
	require.NoError(t, err)
	lockingScriptStr := "d9ad6a3aba0b1cc57071409f3ebc229193647ad43f715e496a91427d6e812c60"
	wocScript, err := hex.DecodeString(lockingScriptStr)
	require.NoError(t, err)
	lockingScript := script.Script(wocScript)
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			utxoClient := &mocks.UtxoClientMock{
				GetBalanceWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (uint64, uint64, error) {
					return 1000, 0, tc.getBalanceWithRetriesErr
				},
				GetUTXOsWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (sdkTx.UTXOs, error) {
					if tc.getUTXOsWithRetriesErr != nil {
						return nil, tc.getUTXOsWithRetriesErr
					}

					utxosToReturn := sdkTx.UTXOs{}
					baseUtxo := sdkTx.UTXOs{
						{
							TxID:          hash1,
							Vout:          0,
							LockingScript: &lockingScript,
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

			client := &mocks.ArcClientMock{}

			sut, err := broadcaster.NewRateBroadcaster(logger,
				client,
				ks,
				utxoClient,
				tc.limit,
				ticker,
				broadcaster.WithBatchSize(tc.batchSize),
				broadcaster.WithIsTestnet(false),
				broadcaster.WithCallback("callbackURL", "callbackToken"),
				broadcaster.WithOpReturn("0"),
				broadcaster.WithFullstatusUpdates(false),
				broadcaster.WithFees(1),
				broadcaster.WithWaitForStatus(metamorph_api.Status_SEEN_ON_NETWORK),
				broadcaster.WithIsTestnet(true),
				broadcaster.WithMaxInputs(1),
			)
			require.NoError(t, err)

			// when then
			err = sut.Initialize()
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedUtxoSetLen, sut.GetUtxoSetLen())
			require.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
			require.Equal(t, int64(0), sut.GetTxCount())
			require.Equal(t, tc.expectedLimit, sut.GetLimit())
			require.Equal(t, int64(0), sut.GetConnectionCount())
			sut.Shutdown()
		})
	}
}

func TestRateBroadcasterStart(t *testing.T) {
	ks, err := keyset.New(&chaincfg.MainNet)
	require.NoError(t, err)

	tt := []struct {
		name                     string
		limit                    int64
		txCount                  int64
		broadcastTransactionsErr error
		sizeJitterMax            int64

		expectedBroadcastTransactionsCalls int
		expectedError                      error
		expectedLimit                      int64
	}{
		{
			name:          "success",
			txCount:       8,
			sizeJitterMax: 1,

			expectedBroadcastTransactionsCalls: 4,
			expectedLimit:                      0,
		},
		{
			name:          "success - limit reached",
			limit:         4,
			txCount:       4,
			sizeJitterMax: 1,

			expectedBroadcastTransactionsCalls: 2,
			expectedLimit:                      4,
		},
		{
			name:                     "error - broadcast transactions",
			broadcastTransactionsErr: errors.New("some error"),
			txCount:                  0,

			expectedBroadcastTransactionsCalls: 4,
			expectedLimit:                      0,
		},
	}

	txIDBytes, _ := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	hash1, err := chainhash.NewHash(txIDBytes)
	require.NoError(t, err)
	lockingScriptStr := "d9ad6a3aba0b1cc57071409f3ebc229193647ad43f715e496a91427d6e812c60"
	wocScript, err := hex.DecodeString(lockingScriptStr)
	require.NoError(t, err)
	lockingScript := script.Script(wocScript)
	for _, tc := range tt {

		tickerCh := make(chan time.Time, 4)
		ticker := &mocks.TickerMock{
			GetTickerChFunc: func() <-chan time.Time {
				return tickerCh
			},
		}

		t.Run(tc.name, func(t *testing.T) {
			// given
			utxoClient := &mocks.UtxoClientMock{
				GetBalanceWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (uint64, uint64, error) {
					return 1000, 0, nil
				},
				GetUTXOsWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (sdkTx.UTXOs, error) {
					utxosToReturn := sdkTx.UTXOs{}
					baseUtxo := sdkTx.UTXOs{
						{
							TxID:          hash1,
							Vout:          0,
							LockingScript: &lockingScript,
							Satoshis:      1000,
						},
					}
					for i := 0; i < 200; i++ {
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

			sut, err := broadcaster.NewRateBroadcaster(logger,
				client,
				ks,
				utxoClient,
				tc.limit,
				ticker,
				broadcaster.WithBatchSize(2),
				broadcaster.WithSizeJitter(tc.sizeJitterMax),
				broadcaster.WithIsTestnet(false),
				broadcaster.WithCallback("callbackURL", "callbackToken"),
				broadcaster.WithOpReturn("0"),
				broadcaster.WithFullstatusUpdates(false),
				broadcaster.WithFees(1),
				broadcaster.WithWaitForStatus(metamorph_api.Status_SEEN_ON_NETWORK),
				broadcaster.WithIsTestnet(true),
				broadcaster.WithMaxInputs(1),
			)
			require.NoError(t, err)

			defer sut.Shutdown()

			// when then
			err = sut.Initialize()
			require.ErrorIs(t, err, tc.expectedError)

			const ticks = 4
			for i := 0; i < ticks; i++ {
				tickerCh <- time.Now()
			}

			sut.Start()

			time.Sleep(1 * time.Second)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedBroadcastTransactionsCalls, len(client.BroadcastTransactionsCalls()))
			assert.Equal(t, tc.txCount, sut.GetTxCount())
			assert.Equal(t, tc.expectedLimit, sut.GetLimit())
			assert.Equal(t, int64(0), sut.GetConnectionCount())
		})
	}
}
