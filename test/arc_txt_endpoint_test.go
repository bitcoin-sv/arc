package test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bitcoinsv/bsvd/bsvec"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Response struct {
	BlockHash   string `json:"blockHash"`
	BlockHeight int    `json:"blockHeight"`
	ExtraInfo   string `json:"extraInfo"`
	Status      int    `json:"status"`
	Timestamp   string `json:"timestamp"`
	Title       string `json:"title"`
	TxStatus    string `json:"txStatus"`
	Txid        string `json:"txid"`
}

type TxStatusResponse struct {
	BlockHash   string      `json:"blockHash"`
	BlockHeight int         `json:"blockHeight"`
	ExtraInfo   interface{} `json:"extraInfo"` // It could be null or any type, so we use interface{}
	Timestamp   string      `json:"timestamp"`
	TxStatus    string      `json:"txStatus"`
	Txid        string      `json:"txid"`
}

func TestHttpPost(t *testing.T) {
	ARC_URL := os.Getenv("ARC_URL")
	t.Logf("testing agains arc url %s", ARC_URL)
	if ARC_URL == "" {
		ARC_URL = "http://localhost:9090"
	}

	address, privateKey := getNewWalletAddress(t)

	generate(t, 100)

	t.Log(address)

	sendToAddress(t, address, 0.001)

	txID := sendToAddress(t, address, 0.02)
	hash := generate(t, 1)

	t.Log(txID)
	t.Log(hash)

	utxos := getUtxos(t, address)
	assert.NotEqualf(t, 0, len(utxos), "No UTXOs available for the address")

	tx := bt.NewTx()

	// Add an input using the first UTXO
	utxo := utxos[0]
	utxoTxID := utxo.Txid
	utxoVout := utxo.Vout
	utxoSatoshis := uint64(utxo.Amount * 1e8) // Convert BTC to satoshis
	utxoScript := utxo.ScriptPubKey

	err := tx.From(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
	require.NoError(t, err)

	// Add an output to the address you've previously created
	recipientAddress := address
	amountToSend := uint64(1) // Example value - 0.009 BTC (taking fees into account)

	recipientScript, err := bscript.NewP2PKHFromAddress(recipientAddress)
	require.NoError(t, err)

	err = tx.PayTo(recipientScript, amountToSend)
	require.NoError(t, err)

	// Sign the input
	wif, err := btcutil.DecodeWIF(privateKey)
	require.NoError(t, err)

	// Extract raw private key bytes directly from the WIF structure
	privateKeyDecoded := wif.PrivKey.Serialize()

	pk, _ := bec.PrivKeyFromBytes(bsvec.S256(), privateKeyDecoded)
	unlockerGetter := unlocker.Getter{PrivateKey: pk}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	require.NoError(t, err)

	extBytes := tx.ExtendedBytes()

	// Print or work with the extended bytes as required
	t.Logf("Extended Bytes: %x\n", extBytes)
	t.Log(extBytes)

	// Convert the transaction bytes to a hex string
	txHexString := hex.EncodeToString(extBytes)

	// Create a JSON object with the rawTx key
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, txHexString)

	url := ARC_URL + "/v1/tx"

	// Create a new request using http.
	req, err := http.NewRequest("POST", url, strings.NewReader(jsonPayload))
	require.NoError(t, err)

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request using http.Client.
	resp, err := http.DefaultClient.Do(req)

	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)

	require.NoError(t, err)

	generate(t, 10)
	i := 0
	for ; i < 5; i++ {
		statusUrl := fmt.Sprintf("%s/v1/tx/%s", ARC_URL, response.Txid)
		statusResp, err := http.Get(statusUrl)

		require.NoError(t, err)
		defer statusResp.Body.Close()

		var statusResponse TxStatusResponse
		err = json.NewDecoder(statusResp.Body).Decode(&statusResponse)
		require.NoError(t, err)
		//TODO: follow up and make sure the tx is MINED
		if statusResponse.TxStatus == "ANNOUNCED_TO_NETWORK" {
			t.Logf("TX %s ANNOUNCED_TO_NETWORK", statusResponse.Txid)
			break
		}

		// Print the extracted txStatus (optional, since you're already asserting it)
		t.Log("Transaction status:", statusResponse.TxStatus)

		time.Sleep(time.Duration(i) * time.Second)
	}

	assert.Less(t, i, 5, "TX status is still not MINED after 5 attempts")
}
