//go:build e2e

package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/node_client"
	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"
)

func TestReorg(t *testing.T) {
	tx1 := prepareTx(t)
	txStale := prepareTx(t)

	// submit tx1
	rawTx, err := tx1.EFHex()
	require.NoError(t, err)
	resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusSeenOnNetwork}, http.StatusOK)
	require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

	// mine tx1
	invHash := node_client.Generate(t, bitcoind, 1)

	statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	statusResp := getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, invHash, *statusResp.BlockHash)

	// invalidate the block with the transaction
	call(t, "invalidateblock", []interface{}{invHash})

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
	require.Equal(t, invHash, *statusResp.BlockHash)
	require.Equal(t, StatusMined, statusResp.TxStatus)

	// make the STALE chain LONGEST by adding a new block
	node_client.Generate(t, bitcoind, 1)

	// verify that stale tx is now MINED
	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, txStale.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, staleHash, *statusResp.BlockHash)

	// verify that previous mined txs have updated block info
	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)
	require.Equal(t, staleHash, *statusResp.BlockHash)
}

func prepareTx(t *testing.T) *transaction.Transaction {
	address, privateKey := node_client.FundNewWallet(t, bitcoind)

	utxos := node_client.GetUtxos(t, bitcoind, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	tx, err := node_client.CreateTx(privateKey, address, utxos[0])
	require.NoError(t, err)

	return tx
}

func call(t *testing.T, method string, params []interface{}) {
	err := node_client.CustomRPCCall(method, params, nodeHost, nodePort, nodeUser, nodePassword)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)
}
