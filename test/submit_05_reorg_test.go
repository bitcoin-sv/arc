//go:build e2e

package test

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
)

func TestReorg(t *testing.T) {
	address, privateKey := node_client.FundNewWallet(t, bitcoind)

	utxos := node_client.GetUtxos(t, bitcoind, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	tx1, err := node_client.CreateTx(privateKey, address, utxos[0])
	require.NoError(t, err)

	// submit tx1
	rawTx, err := tx1.EFHex()
	require.NoError(t, err)
	resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusSeenOnNetwork}, http.StatusOK)
	require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

	// mine tx1
	invHash := node_client.Generate(t, bitcoind, 1)

	// verify tx1 = MINED
	statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	statusResp := getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, invHash, *statusResp.BlockHash)

	// get new UTXO for tx2
	txID := node_client.SendToAddress(t, bitcoind, address, float64(0.002))
	utxos = node_client.GetUtxos(t, bitcoind, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	// make sure to pick the correct UTXO
	var utxo node_client.UnspentOutput
	for _, u := range utxos {
		if u.Txid == txID {
			utxo = u
		}
	}

	tx2, err := node_client.CreateTx(privateKey, address, utxo)
	require.NoError(t, err)
	lis, err := net.Listen("tcp", ":9000")
	require.NoError(t, err)
	mux := http.NewServeMux()
	defer func() {
		err = lis.Close()
		require.NoError(t, err)
	}()

	// prepare a callback server for tx2
	callbackReceivedChan := make(chan *TransactionResponse)
	callbackErrChan := make(chan error)
	callbackURL, token := registerHandlerForCallback(t, callbackReceivedChan, callbackErrChan, nil, mux)
	defer func() {
		t.Log("closing channels")

		close(callbackReceivedChan)
		close(callbackErrChan)
	}()

	go func() {
		t.Logf("starting callback server")
		err = http.Serve(lis, mux)
		if err != nil {
			t.Log("callback server stopped")
		}
	}()

	// submit tx2
	rawTx, err = tx2.EFHex()
	require.NoError(t, err)
	resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}),
		map[string]string{
			"X-WaitFor":       StatusSeenOnNetwork,
			"X-CallbackUrl":   callbackURL,
			"X-CallbackToken": token,
		}, http.StatusOK)
	require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

	// mine tx2
	tx2BlockHash := node_client.Generate(t, bitcoind, 1)

	// verify tx2 = MINED
	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, tx2BlockHash, *statusResp.BlockHash)

	select {
	case status := <-callbackReceivedChan:
		require.Equal(t, tx2.TxID().String(), status.Txid)
		require.Equal(t, StatusMined, status.TxStatus)
	case err := <-callbackErrChan:
		t.Fatalf("callback error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("callback exceeded timeout")
	}

	// invalidate the chain with tx1 and tx2
	call(t, "invalidateblock", []interface{}{invHash})

	// prepare txStale
	txID = node_client.SendToAddress(t, bitcoind, address, float64(0.003))
	utxos = node_client.GetUtxos(t, bitcoind, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	// make sure to pick the correct UTXO
	for _, u := range utxos {
		if u.Txid == txID {
			utxo = u
		}
	}

	txStale, err := node_client.CreateTx(privateKey, address, utxo)
	require.NoError(t, err)

	// post a tx to the STALE chain
	rawTx, err = txStale.EFHex()
	require.NoError(t, err)
	resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusSeenOnNetwork}, http.StatusOK)
	require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

	// post the previously mined tx1 to a STALE chain
	rawTx, err = tx1.EFHex()
	require.NoError(t, err)
	_ = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusSeenOnNetwork}, http.StatusOK)

	// generate new block that will create a stale chain that includes the txStale and tx1
	staleHash := node_client.Generate(t, bitcoind, 1)

	// verify that stale tx is still SEEN_ON_NETWORK
	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, txStale.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusSeenOnNetwork, statusResp.TxStatus)

	// verify that nothing changed so far with previous mined txs
	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, invHash, *statusResp.BlockHash)

	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, tx2BlockHash, *statusResp.BlockHash)

	// make the STALE chain LONGEST by adding 2 new blocks
	node_client.Generate(t, bitcoind, 1)
	node_client.Generate(t, bitcoind, 1)

	// verify that stale tx is now MINED
	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, txStale.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, staleHash, *statusResp.BlockHash)

	// verify that previous mined tx1 have updated block info
	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, staleHash, *statusResp.BlockHash)

	// verify that tx2 was rebroadcasted and is now MINED with new block data
	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID().String())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, tx2BlockHash, *statusResp.BlockHash)
}

func call(t *testing.T, method string, params []interface{}) {
	err := node_client.CustomRPCCall(method, params, nodeHost, nodePort, nodeUser, nodePassword)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)
}
