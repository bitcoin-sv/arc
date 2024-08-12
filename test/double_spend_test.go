package test

import (
	"context"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/libsv/go-bt/v2"
	"github.com/stretchr/testify/require"
)

func TestDoubleSpend(t *testing.T) {
	tt := []struct {
		name      string
		extFormat bool
	}{
		{
			name:      "submit tx with a double spend tx before and after tx got mined - std format",
			extFormat: false,
		},
		{
			name:      "submit tx with a double spend tx before and after tx got mined - ext format",
			extFormat: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			address, privateKey := fundNewWallet(t)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			tx, err := createTx(privateKey, address, utxos[0])
			require.NoError(t, err)

			arcClient, err := api.NewClientWithResponses(arcEndpoint)
			require.NoError(t, err)

			// submit first transaction
			postTxChecksStatus(t, arcClient, tx, Status_ACCEPTED_BY_NETWORK, tc.extFormat)

			// send double spending transaction when first tx is in mempool
			txMempool := createTxToNewAddress(t, privateKey, utxos[0])
			postTxChecksStatus(t, arcClient, txMempool, Status_DOUBLE_SPEND_ATTEMPTED, tc.extFormat)

			var statusResponse *api.GETTransactionStatusResponse
			ctx := context.Background()

			// give arc time to update the status of all competing transactions
			time.Sleep(5 * time.Second)

			// verify that the first tx was also set to DOUBLE_SPEND_ATTEMPTED
			statusResponse, err = arcClient.GETTransactionStatusWithResponse(ctx, tx.TxID())
			require.NoError(t, err)
			if tc.extFormat {
				require.Equal(t, Status_DOUBLE_SPEND_ATTEMPTED, *statusResponse.JSON200.TxStatus)
			} else {
				require.Equal(t, Status_DOUBLE_SPEND_ATTEMPTED, *statusResponse.JSON200.TxStatus)
			}

			// mine the first tx
			generate(t, 10)

			// give arc time to update the status of all competing transactions
			time.Sleep(5 * time.Second)

			// verify that the first tx was mined
			statusResponse, err = arcClient.GETTransactionStatusWithResponse(ctx, tx.TxID())
			require.NoError(t, err)
			if tc.extFormat {
				require.Equal(t, Status_REJECTED, *statusResponse.JSON200.TxStatus)
			} else {
				require.Equal(t, Status_MINED, *statusResponse.JSON200.TxStatus)
			}

			// verify that the second tx was rejected
			statusResponse, err = arcClient.GETTransactionStatusWithResponse(ctx, txMempool.TxID())
			require.NoError(t, err)
			require.Equal(t, Status_REJECTED, *statusResponse.JSON200.TxStatus)
			require.Equal(t, "double spend attempted", *statusResponse.JSON200.ExtraInfo)

			// send double spending transaction when first tx was mined
			txMined := createTxToNewAddress(t, privateKey, utxos[0])
			postTxChecksStatus(t, arcClient, txMined, Status_SEEN_IN_ORPHAN_MEMPOOL, tc.extFormat)
		})
	}
}

func postTxChecksStatus(t *testing.T, client *api.ClientWithResponses, tx *bt.Tx, expectedStatus string, extFormat bool) {
	rawTxString := hex.EncodeToString(tx.Bytes())
	if extFormat {
		rawTxString = hex.EncodeToString(tx.ExtendedBytes())
	}
	body := api.POSTTransactionJSONRequestBody{
		RawTx: rawTxString,
	}

	ctx := context.Background()
	params := &api.POSTTransactionParams{
		XWaitFor: &expectedStatus,
	}
	response, err := client.POSTTransactionWithResponse(ctx, params, body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, response.StatusCode())
	require.NotNil(t, response.JSON200)
	require.Equalf(t, expectedStatus, response.JSON200.TxStatus, "status of response: %s does not match expected status: %s for tx ID %s", response.JSON200.TxStatus, expectedStatus, tx.TxID())
}

func createTxToNewAddress(t *testing.T, privateKey string, utxo NodeUnspentUtxo) *bt.Tx {
	address, err := bitcoind.GetNewAddress()
	require.NoError(t, err)
	tx1, err := createTx(privateKey, address, utxo)
	require.NoError(t, err)

	return tx1
}
