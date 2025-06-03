//go:build e2e

package test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
)

func TestReorg(t *testing.T) {
	fmt.Println("shota TestReorg")
	address, privateKey := node_client.FundNewWallet(t, bitcoind)

	utxos := node_client.GetUtxos(t, bitcoind, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

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

	tx1, err := node_client.CreateTx(privateKey, address, utxos[0])
	require.NoError(t, err)

	// submit tx1
	rawTx, err := tx1.EFHex()
	require.NoError(t, err)
	resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}),
		map[string]string{
			"X-WaitFor":       StatusSeenOnNetwork,
			"X-CallbackUrl":   callbackURL,
			"X-CallbackToken": token,
		}, http.StatusOK)
	require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

	// mine tx1
	invHash := node_client.Generate(t, bitcoind, 1)

	// verify tx1 = MINED
	checkStatusBlockHash(t, tx1.TxID().String(), StatusMined, invHash)

	merklePathTx1 := getMerklePath(t, tx1.TxID().String())

	select {
	case status := <-callbackReceivedChan:
		require.Equal(t, tx1.TxID().String(), status.Txid)
		require.Equal(t, StatusMined, status.TxStatus)
		require.Equal(t, invHash, *status.BlockHash)
		require.Equal(t, merklePathTx1, *status.MerklePath)
	case err := <-callbackErrChan:
		t.Fatalf("callback error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("callback exceeded timeout")
	}

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

	// verify tx2 is MINED
	checkStatusBlockHash(t, tx2.TxID().String(), StatusMined, tx2BlockHash)

	select {
	case status := <-callbackReceivedChan:
		require.Equal(t, tx2.TxID().String(), status.Txid)
		require.Equal(t, StatusMined, status.TxStatus)
		require.Equal(t, tx2BlockHash, *status.BlockHash)
	case err := <-callbackErrChan:
		t.Fatalf("callback error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("callback exceeded timeout")
	}

	// invalidate the chain with tx1 and tx2
	client, err := node_client.NewRPCClient(nodeHost, nodePort, nodeUser, nodePassword)
	require.NoError(t, err)

	err = client.InvalidateBlock(context.TODO(), invHash)
	require.NoError(t, err)

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
	checkStatus(t, txStale.TxID().String(), StatusSeenOnNetwork)

	// verify that nothing changed so far with previous mined txs
	dsa, _ := tx1.EFHex()
	fmt.Println("shota tx1", dsa, tx1.Hex(), tx1.TxID().String())
	checkStatusBlockHash(t, tx1.TxID().String(), StatusMined, invHash)
	fmt.Println("shota tx2", tx2.TxID().String())
	fmt.Println("shota stale", txStale.TxID().String())
	checkStatusBlockHash(t, tx2.TxID().String(), StatusMined, tx2BlockHash)

	// make the STALE chain LONGEST by adding 2 new blocks
	node_client.Generate(t, bitcoind, 1)
	node_client.Generate(t, bitcoind, 1)

	// verify that stale tx is now MINED
	checkStatusBlockHash(t, txStale.TxID().String(), StatusMined, staleHash)

	// verify that previous mined tx1 have updated block info
	checkStatusBlockHash(t, tx1.TxID().String(), StatusMined, staleHash)

	// verify that tx2 is now MINED_IN_STALE_BLOCK
	checkStatusBlockHash(t, tx2.TxID().String(), StatusMined, staleHash)

	merklePathTx1 = getMerklePath(t, tx1.TxID().String())

	// expect 2 callbacks
	for range 2 {
		select {
		case status := <-callbackReceivedChan:
			switch status.Txid {
			// verify that callback for tx2 was received with status MINED_IN_STALE_BLOCK
			case tx2.TxID().String():
				require.Equal(t, StatusMinedInStaleBlock, status.TxStatus)
				require.Equal(t, tx2BlockHash, *status.BlockHash)
			// verify that callback for tx1 was received with status MINED and updated merkle path
			case tx1.TxID().String():
				require.Equal(t, StatusMined, status.TxStatus)
				require.Equal(t, staleHash, *status.BlockHash)
				require.Equal(t, merklePathTx1, *status.MerklePath)
			default:
				t.Fatal("Unexpected tx id")
			}
		case err := <-callbackErrChan:
			t.Fatalf("callback error: %v", err)
		case <-time.After(1 * time.Second):
			t.Fatal("callback exceeded timeout")
		}
	}

	time.Sleep(10 * time.Second) // wait for callbacks to be processed
}
