package broadcaster_test

import (
	"context"
	"os"
	"testing"
	"time"

	"log/slog"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"
)

func TestBroadcaster(t *testing.T) {

	mockedUtxoClient := &mocks.UtxoClientMock{
		GetBalanceFunc: func(ctx context.Context, address string) (int64, int64, error) {
			return 1000, 0, nil
		},
		GetBalanceWithRetriesFunc: func(ctx context.Context, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error) {
			return 1000, 0, nil
		},
		GetUTXOsFunc: func(ctx context.Context, lockingScript *script.Script, address string) (sdkTx.UTXOs, error) {
			return sdkTx.UTXOs{
				{
					TxID:          []byte("sample-txid-1"),
					Vout:          0,
					LockingScript: lockingScript,
					Satoshis:      1000,
				},
				{
					TxID:          []byte("sample-txid-2"),
					Vout:          1,
					LockingScript: lockingScript,
					Satoshis:      1000,
				},
			}, nil // Mock response for UTXOs retrieval with multiple UTXOs
		},
		GetUTXOsWithRetriesFunc: func(ctx context.Context, lockingScript *script.Script, address string, constantBackoff time.Duration, retries uint64) (sdkTx.UTXOs, error) {
			return sdkTx.UTXOs{
				{
					TxID:          []byte("sample-txid-1"),
					Vout:          0,
					LockingScript: lockingScript,
					Satoshis:      1000,
				},
				{
					TxID:          []byte("sample-txid-2"),
					Vout:          1,
					LockingScript: lockingScript,
					Satoshis:      1000,
				},
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ks, err := keyset.New()
	require.NoError(t, err)

	rb, err := broadcaster.NewRateBroadcaster(
		logger,
		nil,
		ks,
		mockedUtxoClient,
		false,
		2,
		50,
		broadcaster.WithBatchSize(2),
	)
	require.NoError(t, err)

	err = rb.Start()
	require.NoError(t, err)

	require.Equal(t, 0, len(mockedUtxoClient.GetBalanceCalls()))
	require.Equal(t, 1, len(mockedUtxoClient.GetBalanceWithRetriesCalls()))
	require.Equal(t, 1, len(mockedUtxoClient.GetUTXOsWithRetriesCalls()))
}
