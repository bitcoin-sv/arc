package test

import (
	"context"
	"encoding/hex"
	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/handler"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
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
			address, privateKey := getNewWalletAddress(t)

			generate(t, 100)

			t.Logf("generated address: %s", address)

			sendToAddress(t, address, 0.001)

			txID := sendToAddress(t, address, 0.02)
			t.Logf("sent 0.02 BSV to: %s", txID)

			hash := generate(t, 1)
			t.Logf("generated 1 block: %s", hash)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			tx, err := createTx(privateKey, address, utxos[0])
			require.NoError(t, err)

			url := "http://arc:9090/"

			arcClient, err := api.NewClientWithResponses(url)
			require.NoError(t, err)

			arcBody := api.POSTTransactionJSONRequestBody{
				RawTx: hex.EncodeToString(tx.ExtendedBytes()),
			}

			//submit first transaction
			postTx(t, arcClient, arcBody, "SEEN_ON_NETWORK")

			// send double spending transaction when first tx is in mempool
			arcBodyMempool := getArcBody(t, privateKey, utxos[0], tc.extFormat)
			postTx(t, arcClient, arcBodyMempool, "REJECTED")

			generate(t, 10)

			ctx := context.Background()
			var statusResponse *api.GETTransactionStatusResponse
			statusResponse, err = arcClient.GETTransactionStatusWithResponse(ctx, tx.TxID())

			require.Equal(t, handler.PtrTo("MINED"), statusResponse.JSON200.TxStatus)

			// send double spending transaction when first tx was mined
			arcBodyMined := getArcBody(t, privateKey, utxos[0], tc.extFormat)
			postTx(t, arcClient, arcBodyMined, "ORPHANED")
		})
	}
}

func postTx(t *testing.T, client *api.ClientWithResponses, body api.POSTTransactionJSONRequestBody, expectedStatus string) {
	ctx := context.Background()
	waitForStatus := api.WaitForStatus(metamorph_api.Status_SEEN_ON_NETWORK)
	params := &api.POSTTransactionParams{
		XWaitForStatus: &waitForStatus,
	}
	response, err := client.POSTTransactionWithResponse(ctx, params, body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, response.StatusCode())
	require.NotNil(t, response.JSON200)
	require.Equalf(t, expectedStatus, response.JSON200.TxStatus, "status of response: %s does not match expected status: %s", response.JSON200.TxStatus, expectedStatus)
}

func getArcBody(t *testing.T, privateKey string, utxo NodeUnspentUtxo, extFormat bool) api.POSTTransactionJSONRequestBody {
	address, err := bitcoind.GetNewAddress()

	tx1, err := createTx(privateKey, address, utxo)
	require.NoError(t, err)
	var arcBodyTx string
	if extFormat {
		arcBodyTx = hex.EncodeToString(tx1.ExtendedBytes())
	} else {
		arcBodyTx = hex.EncodeToString(tx1.Bytes())
	}
	arcBody := api.POSTTransactionJSONRequestBody{
		RawTx: arcBodyTx,
	}
	return arcBody
}
