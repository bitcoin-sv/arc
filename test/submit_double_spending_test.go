//go:build e2e

package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"

	"github.com/stretchr/testify/require"
)

func TestDoubleSpend(t *testing.T) {
	t.Run("submit tx with a double spend tx before and after tx got mined - ext format", func(t *testing.T) {
		address, privateKey := fundNewWallet(t)

		utxos := getUtxos(t, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		tx1, err := createTx(privateKey, address, utxos[0])
		require.NoError(t, err)

		// submit first transaction
		rawTx, err := tx1.EFHex()
		require.NoError(t, err)

		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": Status_SEEN_ON_NETWORK}, http.StatusOK)
		require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)

		// send double spending transaction when first tx is in mempool
		tx2 := createTxToNewAddress(t, privateKey, utxos[0])
		rawTx, err = tx2.EFHex()
		require.NoError(t, err)

		// submit second transaction
		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": Status_DOUBLE_SPEND_ATTEMPTED}, http.StatusOK)
		require.Equal(t, Status_DOUBLE_SPEND_ATTEMPTED, resp.TxStatus)
		require.Equal(t, []string{tx1.TxID()}, *resp.CompetingTxs)

		// give arc time to update the status of all competing transactions
		time.Sleep(5 * time.Second)

		statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
		statusResp := getRequest[TransactionResponse](t, statusUrl)

		// verify that the first tx was also set to DOUBLE_SPEND_ATTEMPTED
		require.Equal(t, Status_DOUBLE_SPEND_ATTEMPTED, statusResp.TxStatus)
		require.Equal(t, []string{tx2.TxID()}, *statusResp.CompetingTxs)

		// mine the first tx
		generate(t, 1)

		// verify that one of the competing transactions was mined, and the other was rejected
		tx1_statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
		tx1_statusResp := getRequest[TransactionResponse](t, tx1_statusUrl)

		tx2_statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
		tx2_statusResp := getRequest[TransactionResponse](t, tx2_statusUrl)

		require.Contains(t, []string{tx1_statusResp.TxStatus, tx2_statusResp.TxStatus}, Status_MINED)
		require.Contains(t, []string{tx1_statusResp.TxStatus, tx2_statusResp.TxStatus}, Status_REJECTED)

		require.Contains(t, []string{*tx1_statusResp.ExtraInfo, *tx2_statusResp.ExtraInfo}, "previously double spend attempted")
		require.Contains(t, []string{*tx1_statusResp.ExtraInfo, *tx2_statusResp.ExtraInfo}, "double spend attempted")

		// send double spending transaction when previous tx was mined
		txMined := createTxToNewAddress(t, privateKey, utxos[0])
		rawTx, err = txMined.EFHex()
		require.NoError(t, err)

		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": Status_SEEN_IN_ORPHAN_MEMPOOL}, http.StatusOK)
		require.Equal(t, Status_SEEN_IN_ORPHAN_MEMPOOL, resp.TxStatus)
	})
}

func createTxToNewAddress(t *testing.T, privateKey string, utxo NodeUnspentUtxo) *sdkTx.Transaction {
	address, err := bitcoind.GetNewAddress()
	require.NoError(t, err)
	tx1, err := createTx(privateKey, address, utxo)
	require.NoError(t, err)

	return tx1
}
