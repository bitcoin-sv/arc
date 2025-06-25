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

		callbackURL, token, callbackReceivedChan, callbackErrChan, cleanup := CreateCallbackServer(t)
		defer cleanup()

		tx1, err := node_client.CreateTx(privateKey, address, utxos[0])
		require.NoError(t, err)

		// submit first transaction
		rawTx1 := tx1.Hex()
		t.Logf("tx 1 ID: %s", tx1.TxID().String())

		// submit conflicting tx1 outside arc
		client, err := rpc_client.NewRPCClient(nodeHost, nodePort, nodeUser, nodePassword)
		require.NoError(t, err)
		tx1ID, err := client.SendRawTransaction(context.Background(), rawTx1)
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
		time.Sleep(5 * time.Second)

		// mine the first tx
		node_client.Generate(t, bitcoind, 3)

		// make sure tx1 is mined in the block
		minedTx, err := client.GetRawTransactionVerbose(context.Background(), tx1ID)
		require.NoError(t, err)
		require.NotEqual(t, "", minedTx.BlockHash)

		time.Sleep(13 * time.Second)

		// verify that one of the competing transactions was mined, and the other was rejected
		tx1StatusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
		tx1StatusResp := getRequest[TransactionResponse](t, tx1StatusURL)

		tx2StatusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
		tx2StatusResp := getRequest[TransactionResponse](t, tx2StatusURL)

		type callbackData struct {
			txID   string
			status string
		}

		var expectedReceivedCallbacks []callbackData

		if tx1StatusResp.TxStatus == StatusMined {
			checkDoubleSpendResponse(t, tx1StatusResp, tx2StatusResp, tx2StatusResp)
			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx1.TxID().String(), status: StatusMined})
			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx2.TxID().String(), status: StatusRejected})
		} else if tx2StatusResp.TxStatus == StatusMined {
			checkDoubleSpendResponse(t, tx2StatusResp, tx1StatusResp, tx1StatusResp)
			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx2.TxID().String(), status: StatusMined})
			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx1.TxID().String(), status: StatusRejected})
		}

		// give arc time to update the status of all competing transactions
		time.Sleep(15 * time.Second)
		// make sure tx1 is mined in the block
		minedTx, err = client.GetRawTransactionVerbose(context.Background(), tx1ID)
		require.NoError(t, err)
		require.NotEqual(t, "", minedTx.BlockHash)
		t.Logf("shotaa %s %s %d ", tx2StatusResp.TxStatus, minedTx.BlockHash, minedTx.Confirmations)

		tx2StatusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
		tx2StatusResp = getRequest[TransactionResponse](t, tx2StatusURL)
		require.Equal(t, StatusRejected, tx2StatusResp.TxStatus)

		// wait for callbacks
		callbackTimeout := time.After(5 * time.Second)

		var receivedCallbacks []callbackData

	callbackLoop:
		for {
			select {
			case status := <-callbackReceivedChan:
				receivedCallbacks = append(receivedCallbacks, callbackData{txID: status.Txid, status: status.TxStatus})
			case err = <-callbackErrChan:
				t.Fatalf("callback error: %v", err)
			case <-callbackTimeout:
				break callbackLoop
			}
		}

		t.Log("expected callbacks", expectedReceivedCallbacks)
		t.Log("received callbacks", receivedCallbacks)
		require.ElementsMatch(t, expectedReceivedCallbacks, receivedCallbacks)

		// send double spending transaction when previous tx was mined
		txMined := createTxToNewAddress(t, privateKey, utxos[0])
		rawTxMined, err := txMined.EFHex()
		require.NoError(t, err)

		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTxMined}), map[string]string{"X-WaitFor": StatusSeenInOrphanMempool}, http.StatusOK)
		require.Equal(t, StatusSeenInOrphanMempool, resp.TxStatus)
	})
}
