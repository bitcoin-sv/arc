package broadcaster_test

import (
	"context"
	"encoding/hex"
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
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

func TestBroadcaster(t *testing.T) {
	txIDbytes, _ := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	hash1, err := chainhash.NewHash(txIDbytes)
	require.NoError(t, err)

	txIDbytes, _ = hex.DecodeString("1a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	hash2, err := chainhash.NewHash(txIDbytes)
	require.NoError(t, err)
	lockingScriptStr := "d9ad6a3aba0b1cc57071409f3ebc229193647ad43f715e496a91427d6e812c60"
	wocScript, err := hex.DecodeString(lockingScriptStr)
	require.NoError(t, err)
	lockingScript := script.Script(wocScript)

	// given
	mockedUtxoClient := &mocks.UtxoClientMock{
		GetBalanceFunc: func(_ context.Context, _ string) (uint64, uint64, error) {
			return 1000, 0, nil
		},
		GetBalanceWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (uint64, uint64, error) {
			return 1000, 0, nil
		},
		GetUTXOsFunc: func(_ context.Context, _ string) (sdkTx.UTXOs, error) {
			return sdkTx.UTXOs{
				{
					TxID:          hash1,
					Vout:          0,
					LockingScript: &lockingScript,
					Satoshis:      1000,
				},
				{
					TxID:          hash2,
					Vout:          1,
					LockingScript: &lockingScript,
					Satoshis:      1000,
				},
			}, nil // Mock response for UTXOs retrieval with multiple UTXOs
		},
		GetUTXOsWithRetriesFunc: func(_ context.Context, _ string, _ time.Duration, _ uint64) (sdkTx.UTXOs, error) {
			return sdkTx.UTXOs{
				{
					TxID:          hash1,
					Vout:          0,
					LockingScript: &lockingScript,
					Satoshis:      1000,
				},
				{
					TxID:          hash2,
					Vout:          1,
					LockingScript: &lockingScript,
					Satoshis:      1000,
				},
			}, nil
		},
	}

	arcClient := &mocks.ArcClientMock{
		BroadcastTransactionsFunc: func(_ context.Context, _ sdkTx.Transactions, _ metamorph_api.Status, _ string, _ string, _ bool, _ bool) ([]*metamorph_api.TransactionStatus, error) {
			return nil, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ks, err := keyset.New(&chaincfg.MainNet)
	require.NoError(t, err)

	ticker := &mocks.TickerMock{
		GetTickerChFunc: func() <-chan time.Time {
			tickerCh := make(chan time.Time)
			return tickerCh
		},
	}

	sut, err := broadcaster.NewRateBroadcaster(
		logger,
		arcClient,
		ks,
		mockedUtxoClient,
		2,
		ticker,
		broadcaster.WithBatchSize(2),
		broadcaster.WithWaitForStatus(metamorph_api.Status_SEEN_ON_NETWORK),
		broadcaster.WithFees(uint64(1)),
		broadcaster.WithFullstatusUpdates(true),
		broadcaster.WithCallback(api.CallbackUrl("someurl"), "token"),
		broadcaster.WithOpReturn("op"),
		broadcaster.WithSizeJitter(1000),
		broadcaster.WithIsTestnet(false),
	)
	require.NoError(t, err)

	// when
	actualError := sut.Initialize(context.TODO(), 20)

	// then
	require.NoError(t, actualError)
	require.Equal(t, 0, len(mockedUtxoClient.GetBalanceCalls()))
	require.Equal(t, 1, len(mockedUtxoClient.GetBalanceWithRetriesCalls()))
	require.Equal(t, 1, len(mockedUtxoClient.GetUTXOsWithRetriesCalls()))
}
