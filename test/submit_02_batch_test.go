//go:build e2e

package test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
)

func TestBatchChainedTxs(t *testing.T) {
	if os.Getenv("TEST_LOCAL_MCAST") != "" {
		t.Skip("Multicasting does't support chained txs yet")
	}

	t.Run("submit batch of chained transactions", func(t *testing.T) {
		address, privateKey := node_client.FundNewWallet(t, bitcoind)

		utxos := node_client.GetUtxos(t, bitcoind, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		txs, err := node_client.CreateTxChain(privateKey, utxos[0], 20)
		require.NoError(t, err)

		request := make([]TransactionRequest, len(txs))
		for i, tx := range txs {
			rawTx, err := tx.EFHex()
			require.NoError(t, err)
			request[i] = TransactionRequest{
				RawTx: rawTx,
			}
		}

		// Send POST request
		t.Logf("submitting batch of %d chained txs", len(txs))
		resp := postRequest[TransactionResponseBatch](t, arcEndpointV1Txs, createPayload(t, request), nil, http.StatusOK)
		hasFailed := false
		for i, txResponse := range resp {
			if !assert.Equal(t, StatusSeenOnNetwork, txResponse.TxStatus, fmt.Sprintf("index: %d", i)) {
				hasFailed = true
			}
		}
		if hasFailed {
			t.FailNow()
		}

		time.Sleep(1 * time.Second)

		// repeat request to ensure response remains the same
		t.Logf("re-submitting batch of %d chained txs", len(txs))
		resp = postRequest[TransactionResponseBatch](t, arcEndpointV1Txs, createPayload(t, request), nil, http.StatusOK)
		for i, txResponse := range resp {
			if !assert.Equal(t, StatusSeenOnNetwork, txResponse.TxStatus, fmt.Sprintf("index: %d", i)) {
				hasFailed = true
			}
		}
		if hasFailed {
			t.FailNow()
		}
	})
}
