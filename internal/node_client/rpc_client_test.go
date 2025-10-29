package node_client_test

import (
	"context"
	"testing"

	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
	"github.com/bitcoin-sv/arc/pkg/rpc_client"
)

func TestRPCClient(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	setup()
	sut, err := rpc_client.NewRPCClient(host, hostPort, user, password)
	require.NoError(t, err)

	address, _ := node_client.FundNewWallet(t, bitcoind)

	utxos := node_client.GetUtxos(t, bitcoind, address)
	require.GreaterOrEqual(t, len(utxos), 1, "No UTXOs available for the address")

	testutils.RunParallel(t, true, "invalidate block", func(t *testing.T) {
		// given
		blockHash, err := bitcoind.Generate(1)
		require.NoError(t, err)

		// when
		err = sut.InvalidateBlock(ctx, blockHash[0])

		// then
		require.NoError(t, err)

		// given
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		// when
		err = sut.InvalidateBlock(cancelCtx, blockHash[0])

		// then
		require.ErrorIs(t, err, context.Canceled)
	})
}
