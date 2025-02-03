//go:build e2e

package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
)

func TestDoubleSpend(t *testing.T) {
	t.Run("submit tx with a double spend tx before and after tx got mined - ext format", func(t *testing.T) {
		address, privateKey := node_client.FundNewWallet(t, bitcoind)

		utxos := node_client.GetUtxos(t, bitcoind, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		tx1, err := node_client.CreateTx(privateKey, address, utxos[0])
		require.NoError(t, err)

		// submit first transaction
		rawTx, err := tx1.EFHex()
		require.NoError(t, err)

		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusSeenOnNetwork}, http.StatusOK)
		require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

		// send double spending transaction when first tx is in mempool
		tx2 := createTxToNewAddress(t, privateKey, utxos[0])
		rawTx, err = tx2.EFHex()
		require.NoError(t, err)

		// submit second transaction
		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusDoubleSpendAttempted}, http.StatusOK)
		require.Equal(t, StatusDoubleSpendAttempted, resp.TxStatus)
		require.Equal(t, []string{tx1.TxID().String()}, *resp.CompetingTxs)

		// give arc time to update the status of all competing transactions
		time.Sleep(5 * time.Second)

		statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
		statusResp := getRequest[TransactionResponse](t, statusURL)

		// verify that the first tx was also set to DOUBLE_SPEND_ATTEMPTED
		require.Equal(t, StatusDoubleSpendAttempted, statusResp.TxStatus)
		require.Equal(t, []string{tx2.TxID().String()}, *statusResp.CompetingTxs)

		// mine the first tx
		node_client.Generate(t, bitcoind, 1)

		// verify that one of the competing transactions was mined, and the other was rejected
		tx1StatusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
		tx1StatusResp := getRequest[TransactionResponse](t, tx1StatusURL)

		tx2StatusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
		tx2StatusResp := getRequest[TransactionResponse](t, tx2StatusURL)

		require.Contains(t, []string{tx1StatusResp.TxStatus, tx2StatusResp.TxStatus}, StatusMined)
		require.Contains(t, []string{tx1StatusResp.TxStatus, tx2StatusResp.TxStatus}, StatusRejected)

		require.Contains(t, []string{*tx1StatusResp.ExtraInfo, *tx2StatusResp.ExtraInfo}, "previously double spend attempted")
		require.Contains(t, []string{*tx1StatusResp.ExtraInfo, *tx2StatusResp.ExtraInfo}, "double spend attempted")

		// send double spending transaction when previous tx was mined
		txMined := createTxToNewAddress(t, privateKey, utxos[0])
		rawTx, err = txMined.EFHex()
		require.NoError(t, err)

		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusSeenInOrphanMempool}, http.StatusOK)
		require.Equal(t, StatusSeenInOrphanMempool, resp.TxStatus)
	})
}

func createTxToNewAddress(t *testing.T, privateKey string, utxo node_client.UnspentOutput) *sdkTx.Transaction {
	address, err := bitcoind.GetNewAddress()
	require.NoError(t, err)
	tx1, err := node_client.CreateTx(privateKey, address, utxo)
	require.NoError(t, err)

	return tx1
}
