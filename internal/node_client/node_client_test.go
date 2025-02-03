package node_client_test

import (
	"context"
	"log"
	"os"
	"strconv"
	"testing"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/ordishs/go-bitcoin"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
)

var (
	hostPort int
	bitcoind *bitcoin.Bitcoind
)

const (
	host     = "localhost"
	port     = 18332
	user     = "bitcoin"
	password = "bitcoin"
)

func TestMain(m *testing.M) {
	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	cmds := []string{"/entrypoint.sh", "bitcoind"}

	resource, resourcePort, err := testutils.RunNode(pool, strconv.Itoa(port), "node", cmds...)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		err = pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	hostPort, err = strconv.Atoi(resourcePort)
	if err != nil {
		log.Fatalf("failed to convert port to int: %v", err)
	}

	return m.Run()
}

func setup() {
	log.Printf("init tests")

	var err error
	bitcoind, err = bitcoin.New(host, hostPort, user, password, false)
	if err != nil {
		log.Fatalln("Failed to create bitcoind instance:", err)
	}

	_, err = bitcoind.GetInfo()
	if err != nil {
		log.Fatalln(err)
	}

	// fund node
	const minNumbeOfBlocks = 101
	_, err = bitcoind.Generate(minNumbeOfBlocks)
	if err != nil {
		log.Fatalln(err)
	}
}

func TestNodeClient(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	setup()

	sut, err := node_client.New(bitcoind)
	require.NoError(t, err)

	address, privateKey := node_client.FundNewWallet(t, bitcoind)

	utxos := node_client.GetUtxos(t, bitcoind, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	t.Run("get raw transaction", func(t *testing.T) {
		// given
		var rawTx *sdkTx.Transaction

		// when
		rawTx, err = sut.GetRawTransaction(ctx, utxos[0].Txid)

		// then
		require.NoError(t, err)

		require.Equal(t, utxos[0].Txid, rawTx.TxID().String())

		// when
		_, err = sut.GetRawTransaction(ctx, "not areal id")

		// then
		require.ErrorIs(t, err, node_client.ErrFailedToGetRawTransaction)
	})

	t.Run("get mempool ancestors", func(t *testing.T) {
		// given
		txs, err := node_client.CreateTxChain(privateKey, utxos[0], 20)
		require.NoError(t, err)

		expectedTxIDs := make([]string, len(txs))
		for i, tx := range txs {
			_, err := bitcoind.SendRawTransaction(tx.String())
			require.NoError(t, err)

			if i != len(txs) {
				expectedTxIDs[i] = tx.TxID().String()
			}
		}

		// when
		ancestorTxIDs, err := sut.GetMempoolAncestors(ctx, []string{txs[len(txs)-1].TxID().String()})

		// then
		require.NoError(t, err)
		require.Len(t, ancestorTxIDs, len(txs))

		// compare expected and actual ancestor tx IDs while ignoring the order
		less := func(a, b string) bool { return a < b }
		equalIgnoreOrder := cmp.Diff(expectedTxIDs, ancestorTxIDs, cmpopts.SortSlices(less)) == ""

		require.True(t, equalIgnoreOrder)

		// when
		_, err = sut.GetMempoolAncestors(ctx, []string{"not a real id"})

		// then
		require.ErrorIs(t, err, node_client.ErrFailedToGetMempoolAncestors)

		// when
		actual, err := sut.GetMempoolAncestors(ctx, []string{"792bb5c0d5e4937572e6368dc713ba0b4935d27a7b7ac654c2b384d6c8b2fb89"}) // tx id not existing in the mempool

		// then
		require.NoError(t, err)
		require.Empty(t, actual)
	})
}
