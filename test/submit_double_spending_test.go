package test

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/libsv/go-bt/v2"
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
		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}), map[string]string{"X-WaitFor": Status_SEEN_ON_NETWORK}, http.StatusOK)
		require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)

		// send double spending transaction when first tx is in mempool
		txMempool := createTxToNewAddress(t, privateKey, utxos[0])

		// submit first transaction
		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(txMempool.ExtendedBytes())}), map[string]string{"X-WaitFor": Status_DOUBLE_SPEND_ATTEMPTED}, http.StatusOK)
		require.Equal(t, Status_DOUBLE_SPEND_ATTEMPTED, resp.TxStatus)

		// give arc time to update the status of all competing transactions
		time.Sleep(5 * time.Second)

		statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx.TxID())
		statusResp := getRequest[TransactionResponse](t, statusUrl)
		// verify that the first tx was also set to DOUBLE_SPEND_ATTEMPTED

		require.Equal(t, Status_DOUBLE_SPEND_ATTEMPTED, statusResp.TxStatus)

		// mine the first tx
		generate(t, 10)

		// give arc time to update the status of all competing transactions
		time.Sleep(5 * time.Second)

		statusUrl = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx.TxID())
		statusResp = getRequest[TransactionResponse](t, statusUrl)
		// verify that the first tx was mined
		require.Equal(t, Status_MINED, statusResp.TxStatus)

		statusUrl = fmt.Sprintf("%s/%s", arcEndpointV1Tx, txMempool.TxID())
		statusResp = getRequest[TransactionResponse](t, statusUrl)
		// verify that the second tx was rejected
		require.Equal(t, Status_REJECTED, statusResp.TxStatus)
		require.Equal(t, "double spend attempted", *statusResp.ExtraInfo)

		// send double spending transaction when first tx was mined
		txMined := createTxToNewAddress(t, privateKey, utxos[0])
		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(txMined.ExtendedBytes())}), map[string]string{"X-WaitFor": Status_SEEN_IN_ORPHAN_MEMPOOL}, http.StatusOK)
		require.Equal(t, Status_SEEN_IN_ORPHAN_MEMPOOL, resp.TxStatus)
	})
}

func createTxToNewAddress(t *testing.T, privateKey string, utxo NodeUnspentUtxo) *bt.Tx {
	address, err := bitcoind.GetNewAddress()
	require.NoError(t, err)
	tx1, err := createTx(privateKey, address, utxo)
	require.NoError(t, err)

	return tx1
}
