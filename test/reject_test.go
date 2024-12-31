//go:build e2e

package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/node_client"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/require"
)

func TestRejectTx(t *testing.T) {
	t.Run("A transaction that spends an output which has been spent in an already mined block\n", func(t *testing.T) {
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

		// give arc time to update the status
		time.Sleep(5 * time.Second)

		fmt.Println("<-- First TX before mining", tx1.TxID())
		GetTxMempoolInfo(tx1.TxID(), bitcoind)
		CallRPCCommands()

		// mine the first tx
		node_client.Generate(t, bitcoind, 1)

		fmt.Println("<-- First TX after mining", tx1.TxID())
		GetTxMempoolInfo(tx1.TxID(), bitcoind)
		CallRPCCommands()

		// send double spending transaction when first tx is in mined
		tx2 := createTxToNewAddress(t, privateKey, utxos[0])
		rawTx, err = tx2.EFHex()
		require.NoError(t, err)

		// submit second transaction
		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusDoubleSpendAttempted}, http.StatusOK)
		fmt.Println("Second TX submitted", tx2.TxID())
		fmt.Println("Response: ", resp)
		CallRPCCommands()

		// give arc time to update the status of all competing transactions
		time.Sleep(5 * time.Second)

		fmt.Println("<-- Second TX before mining", tx2.TxID())
		GetTxMempoolInfo(tx2.TxID(), bitcoind)
		CallRPCCommands()

		// try to mine the second tx
		node_client.Generate(t, bitcoind, 1)

		fmt.Println("<-- Second TX after mining", tx2.TxID())
		GetTxMempoolInfo(tx2.TxID(), bitcoind)
		CallRPCCommands()
	})

	//t.Run("A transaction that spends an output of a parent which wasn’t sent yet => missing inputs\n", func(t *testing.T) {
	//	fmt.Println("<- start ->")
	//	CallRPCCommands()
	//
	//	address, privateKey := node_client.FundNewWallet(t, bitcoind)
	//
	//	utxos := node_client.GetUtxos(t, bitcoind, address)
	//	require.True(t, len(utxos) > 0, "No UTXOs available for the address")
	//
	//	tx1, err := node_client.CreateTx(privateKey, address, utxos[0])
	//	require.NoError(t, err)
	//
	//	fmt.Println("<-- First TX without sending to ARC", tx1.TxID())
	//	GetTxMempoolInfo(tx1.TxID(), bitcoind)
	//
	//	// create tx with created (not send) tx
	//	tx2, err := node_client.CreateTxFromTx(privateKey, address, tx1)
	//	require.NoError(t, err)
	//	rawTx, err := tx2.EFHex()
	//	require.NoError(t, err)
	//
	//	fmt.Println("<-- Second TX without sending to ARC", tx2.TxID())
	//	CallRPCCommands()
	//
	//	// submit second transaction
	//	resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}), map[string]string{"X-WaitFor": StatusDoubleSpendAttempted}, http.StatusOK)
	//	fmt.Println("Second TX submitted", tx2.TxID())
	//	fmt.Println("Response: ", resp)
	//
	//	CallRPCCommands()
	//
	//	// give arc time to update the status of all competing transactions
	//	time.Sleep(5 * time.Second)
	//
	//	fmt.Println("<-- First TX before mining", tx1.TxID())
	//	GetTxMempoolInfo(tx1.TxID(), bitcoind)
	//
	//	fmt.Println("<-- Second TX before mining", tx2.TxID())
	//	GetTxMempoolInfo(tx2.TxID(), bitcoind)
	//
	//	CallRPCCommands()
	//
	//	// try to mine the second tx
	//	node_client.Generate(t, bitcoind, 1)
	//
	//	fmt.Println("<-- First TX after mining", tx1.TxID())
	//	GetTxMempoolInfo(tx1.TxID(), bitcoind)
	//
	//	fmt.Println("<-- Second TX after mining", tx2.TxID())
	//	GetTxMempoolInfo(tx2.TxID(), bitcoind)
	//
	//	CallRPCCommands()
	//
	//	// get tx1 status
	//	statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
	//	statusResp := getRequest[TransactionResponse](t, statusURL)
	//
	//	fmt.Println("First TX status", statusResp.TxStatus)
	//	fmt.Println("Whole response", statusResp)
	//
	//	// get tx2 status
	//	statusURL = fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
	//	statusResp = getRequest[TransactionResponse](t, statusURL)
	//
	//	fmt.Println("Second TX status", statusResp.TxStatus)
	//	fmt.Println("Whole response", statusResp)
	//
	//	CallRPCCommands()
	//})
}

func GetTxMempoolInfo(txID string, bitcoind *bitcoin.Bitcoind) {
	// get rawTx by txID
	rawTx, err := bitcoind.GetRawTransaction(txID)
	if err != nil {
		fmt.Println("GetRawTransaction - error getting raw tx", err)
	} else {
		fmt.Println("MEMPOOL INFO - GetRawTransaction: rawTx", rawTx.Hex)
	}

	// get tx info
	txInfo, err := bitcoind.GetMempoolEntry(txID)
	if err != nil {
		fmt.Println("GetMempoolEntry - error getting tx info", err)
	} else {
		fmt.Println("MEMPOOL INFO - GetMempoolEntry: txInfo", txInfo)
	}

	ids := make([]string, 0)
	rawMempool, err := bitcoind.GetRawMempool(true)
	if err != nil {
		fmt.Println("GetRawMempool - error getting mempool", err)
	} else {
		for _, id := range rawMempool {
			ids = append(ids, string(id))
		}
	}
	fmt.Println("MEMPOOL INFO - GetRawMempool: txIDs", len(ids))
}

func CallRPCCommands() {
	err := node_client.CustomRPCCall("getorphaninfo", nil, nodeHost, nodePort, "bitcoin", "bitcoin")
	if err != nil {
		fmt.Println("RPC call error", err)
	}

	err = node_client.CustomRPCCall("getrawnonfinalmempool", nil, nodeHost, nodePort, "bitcoin", "bitcoin")
	if err != nil {
		fmt.Println("RPC call error", err)
	}
}

func createTxToNewAddress(t *testing.T, privateKey string, utxo node_client.UnspentOutput) *sdkTx.Transaction {
	address, err := bitcoind.GetNewAddress()
	require.NoError(t, err)
	tx1, err := node_client.CreateTx(privateKey, address, utxo)
	require.NoError(t, err)

	return tx1
}
