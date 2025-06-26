//go:build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
	"github.com/bitcoin-sv/arc/pkg/rpc_client"
)

func TestDoubleSpendRejected(t *testing.T) {
	t.Run("submit tx with a double spend tx before and after tx got mined - ext format", func(t *testing.T) {
		address, privateKey := node_client.FundNewWallet(t, bitcoind)

		utxos := node_client.GetUtxos(t, bitcoind, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		callbackURL, token, _, _, cleanup := CreateCallbackServer(t)
		defer cleanup()

		tx1, err := node_client.CreateTx(privateKey, address, utxos[0])
		require.NoError(t, err)

		// submit conflicting tx1 outside arc
		client, err := rpc_client.NewRPCClient(nodeHost, nodePort, nodeUser, nodePassword)
		require.NoError(t, err)
		tx1ID, err := client.SendRawTransaction(context.Background(), tx1.Hex())
		require.NoError(t, err)
		t.Logf("tx 1 ID from node: %s", tx1ID)

		// send double spending transaction when first tx is in mempool
		tx2 := createTxToNewAddress(t, privateKey, utxos[0])
		rawTx2, err := tx2.EFHex()
		require.NoError(t, err)
		t.Logf("tx 2 ID: %s", tx2.TxID().String())

		// submit second transaction
		postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx2}),
			map[string]string{
				"X-WaitFor":       StatusDoubleSpendAttempted,
				"X-CallbackUrl":   callbackURL,
				"X-CallbackToken": token,
			},
			http.StatusOK)

		// give arc time to update the status of all competing transactions
		time.Sleep(3 * time.Second)

		// mine the first tx
		node_client.Generate(t, bitcoind, 3)

		// make sure tx1 is mined in the block
		minedTx, err := client.GetRawTransactionVerbose(context.Background(), tx1ID)
		require.NoError(t, err)
		require.NotEqual(t, "", minedTx.BlockHash)

		// check that we have double spend status
		tx2StatusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
		tx2StatusResp := getRequest[TransactionResponse](t, tx2StatusURL)
		require.Equal(t, StatusDoubleSpendAttempted, tx2StatusResp.TxStatus)

		// give arc time to update the status of the second transaction
		time.Sleep(8 * time.Second)

		// make sure the status was updated to rejected
		tx2StatusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
		tx2StatusResp = getRequest[TransactionResponse](t, tx2StatusURL)
		require.Equal(t, StatusRejected, tx2StatusResp.TxStatus)
	})
}
