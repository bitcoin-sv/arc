package test

import (
	"fmt"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDoubleSpend(t *testing.T) {
	t.Run("submit tx with a double spend tx before and after tx got mined - ext format", func(t *testing.T) {
		address, privateKey := fundNewWallet(t)

		utxos := getUtxos(t, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		tx, err := createTx(privateKey, address, utxos[0])
		require.NoError(t, err)

		// submit first transaction
		rawTx, err := tx.EFHex()
		require.NoError(t, err)

		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": Status_SEEN_ON_NETWORK}, http.StatusOK)
		require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)

		// send double spending transaction when first tx is in mempool
		txMempool := createTxToNewAddress(t, privateKey, utxos[0])
		rawTx, err = txMempool.EFHex()
		require.NoError(t, err)

		// submit second transaction
		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": Status_DOUBLE_SPEND_ATTEMPTED}, http.StatusOK)
		require.Equal(t, Status_DOUBLE_SPEND_ATTEMPTED, resp.TxStatus)
		require.Equal(t, []string{tx.TxID()}, *resp.CompetingTxs)

		// give arc time to update the status of all competing transactions
		time.Sleep(5 * time.Second)

		statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx.TxID())
		statusResp := getRequest[TransactionResponse](t, statusUrl)

		// verify that the first tx was also set to DOUBLE_SPEND_ATTEMPTED
		require.Equal(t, Status_DOUBLE_SPEND_ATTEMPTED, statusResp.TxStatus)
		require.Equal(t, []string{txMempool.TxID()}, *statusResp.CompetingTxs)

		// mine the first tx
		generate(t, 10)

		// give arc time to update the status of all competing transactions
		time.Sleep(5 * time.Second)

		// verify that the first tx was mined
		statusUrl = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx.TxID())
		statusResp = getRequest[TransactionResponse](t, statusUrl)
		require.Equal(t, Status_MINED, statusResp.TxStatus)
		require.Equal(t, "previously double spend attempted", *statusResp.ExtraInfo)

		// verify that the second tx was rejected
		statusUrl = fmt.Sprintf("%s/%s", arcEndpointV1Tx, txMempool.TxID())
		statusResp = getRequest[TransactionResponse](t, statusUrl)
		require.Equal(t, Status_REJECTED, statusResp.TxStatus)
		require.Equal(t, "double spend attempted", *statusResp.ExtraInfo)

		// send double spending transaction when first tx was mined
		txMined := createTxToNewAddress(t, privateKey, utxos[0])
		rawTx, err = txMined.EFHex()
		require.NoError(t, err)

		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": Status_SEEN_IN_ORPHAN_MEMPOOL}, http.StatusOK)
		require.Equal(t, Status_SEEN_IN_ORPHAN_MEMPOOL, resp.TxStatus)
	})
}

func createTxToNewAddress(t *testing.T, privateKey string, utxo NodeUnspentUtxo) *transaction.Transaction {
	address, err := bitcoind.GetNewAddress()
	require.NoError(t, err)
	tx1, err := createTx(privateKey, address, utxo)
	require.NoError(t, err)

	return tx1
}
