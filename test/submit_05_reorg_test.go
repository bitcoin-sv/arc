package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/node_client"
	"github.com/stretchr/testify/require"
)

func TestReorg(t *testing.T) {
	invHash := node_client.Generate(t, bitcoind, 1)
	time.Sleep(5 * time.Second)

	address, privateKey := node_client.FundNewWallet(t, bitcoind)

	utxos := node_client.GetUtxos(t, bitcoind, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	tx1, err := node_client.CreateTx(privateKey, address, utxos[0])
	require.NoError(t, err)

	// submit first transaction
	rawTx, err := tx1.EFHex()
	require.NoError(t, err)

	resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusSeenOnNetwork}, http.StatusOK)
	require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

	node_client.Generate(t, bitcoind, 1)

	statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	statusResp := getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMined, statusResp.TxStatus)

	prepareForks(t, invHash)
	time.Sleep(5 * time.Second)

	// statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	// statusResp = getRequest[TransactionResponse](t, statusURL)
	// require.Equal(t, StatusRejected, statusResp.TxStatus)

	node_client.Generate(t, bitcoind, 50) // this should become the longest chain through reorg
	time.Sleep(5 * time.Second)

	// statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	// statusResp = getRequest[TransactionResponse](t, statusURL)
	// require.Equal(t, StatusMined, statusResp.TxStatus)
	//
	// err = node_client.CustomRPCCall("reconsiderblock", []interface{}{invHash}, nodeHost, nodePort, nodeUser, nodePassword)
	// require.NoError(t, err)
	// time.Sleep(5 * time.Second)

	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	statusResp = getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, StatusMinedInStaleBlock, statusResp.TxStatus)
}

func prepareForks(t *testing.T, blockhash string) {
	err := node_client.CustomRPCCall("invalidateblock", []interface{}{blockhash}, nodeHost, nodePort, nodeUser, nodePassword)
	require.NoError(t, err)
}
