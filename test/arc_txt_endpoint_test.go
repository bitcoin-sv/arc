package test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"testing"

	"github.com/bitcoinsv/bsvd/bsvec"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
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

func TestMain(m *testing.M) {
	info, err := bitcoind.GetInfo()
	if err != nil {
		log.Fatalf("failed to get info: %v", err)
	}

	log.Printf("current block height: %d", info.Blocks)

	os.Exit(m.Run())
}

func TestHttpPost(t *testing.T) {
	address, privateKey := getNewWalletAddress(t)

	generate(t, 100, address)

	fmt.Println(address)

	sendToAddress(t, address, 0.001)

	txID := sendToAddress(t, address, 0.02)
	hash := generate(t, 1, address)

	fmt.Println(txID)
	fmt.Println(hash)

	utxos := getUnspentUtxos(t, address)
	if len(utxos) == 0 {
		log.Fatal("No UTXOs available for the address")
	}

	tx := bt.NewTx()

	// Add an input using the first UTXO
	utxo := utxos[0]
	utxoTxID := utxo.Txid
	utxoVout := uint32(utxo.Vout)
	utxoSatoshis := uint64(utxo.Amount * 1e8) // Convert BTC to satoshis
	utxoScript := utxo.ScriptPubKey

	err := tx.From(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
	if err != nil {
		log.Fatalf("Failed adding input: %v", err)
	}

	// Add an output to the address you've previously created
	recipientAddress := address
	amountToSend := uint64(1) // Example value - 0.009 BTC (taking fees into account)

	recipientScript, err := bscript.NewP2PKHFromAddress(recipientAddress)
	if err != nil {
		log.Fatalf("Failed converting address to script: %v", err)
	}

	err = tx.PayTo(recipientScript, amountToSend)
	if err != nil {
		log.Fatalf("Failed adding output: %v", err)
	}

	// Sign the input

	wif, err := btcutil.DecodeWIF(privateKey)
	if err != nil {
		log.Fatalf("Failed to decode WIF: %v", err)
	}

	// Extract raw private key bytes directly from the WIF structure
	privateKeyDecoded := wif.PrivKey.Serialize()

	pk, _ := bec.PrivKeyFromBytes(bsvec.S256(), privateKeyDecoded)
	unlockerGetter := unlocker.Getter{PrivateKey: pk}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		log.Fatalf("sign failed: %v", err)
	}

	extBytes := tx.ExtendedBytes()

	// Print or work with the extended bytes as required
	fmt.Printf("Extended Bytes: %x\n", extBytes)
	fmt.Println(extBytes)

	// Convert the transaction bytes to a hex string
	txHexString := hex.EncodeToString(extBytes)

	// Create a JSON object with the rawTx key
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, txHexString)

	url := "http://arc:9090/v1/tx"

	// The request body data.
	// data := []byte("{}")

	// Create a new request using http.
	req, err := http.NewRequest("POST", url, strings.NewReader(jsonPayload))

	// If there is an error while creating the request, fail the test.
	if err != nil {
		t.Fatalf("Error creating HTTP request: %s", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request using http.Client.
	client := &http.Client{}
	resp, err := client.Do(req)

	// If there is an error while sending the request, fail the test.
	if err != nil {
		t.Fatalf("Error sending HTTP request: %s", err)
	}

	defer resp.Body.Close()

	// If status is not http.StatusOK, then read and print the response body
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %s", err)
	}

	// Print the response body for every request
	fmt.Println("Response body:", string(bodyBytes))

	// If status is not http.StatusOK, then provide an error for the test
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Received status: %s. Response body: %s", resp.Status, string(bodyBytes))
	}

	var response Response
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		t.Fatalf("Failed to decode the response body: %v", err)
	}

	generate(t, 10, recipientAddress)

	time.Sleep(450 * time.Second)

	statusUrl := fmt.Sprintf("http://arc:9090/v1/tx/%s", response.Txid)
	statusResp, err := http.Get(statusUrl)
	if err != nil {
		t.Fatalf("Error sending GET request to /v1/tx/{txid}: %s", err)
	}
	defer statusResp.Body.Close()

	statusBodyBytes, err := ioutil.ReadAll(statusResp.Body)
	if err != nil {
		t.Fatalf("Error reading status response body: %s", err)
	}

	// Print the response body for the GET request
	fmt.Println("Transaction status response body:", string(statusBodyBytes))

	// Unmarshal the status response
	var statusResponse TxStatusResponse
	if err := json.Unmarshal(statusBodyBytes, &statusResponse); err != nil {
		t.Fatalf("Failed to decode the status response body: %v", err)
	}

	// Assert that txStatus is "SEEN_ON_NETWORK"
	if statusResponse.TxStatus != "MINED" {
		t.Fatalf("Expected txStatus to be 'MINED', but got '%s'", statusResponse.TxStatus)
	}

	// Print the extracted txStatus (optional, since you're already asserting it)
	fmt.Println("Transaction status:", statusResponse.TxStatus)

	time.Sleep(20 * time.Second)

	if err = json.Unmarshal(bodyBytes, &response); err != nil { // <-- Use "=" instead of ":="
		t.Fatalf("Failed to decode the response body: %v", err)
	}

	statusResp, err = http.Get(statusUrl) // <-- Use "=" instead of ":="
	if err != nil {
		t.Fatalf("Error sending GET request to /v1/tx/{txid}: %s", err)
	}
	defer statusResp.Body.Close()

	statusBodyBytes, err = ioutil.ReadAll(statusResp.Body) // <-- Use "=" instead of ":="
	if err != nil {
		t.Fatalf("Error reading status response body: %s", err)
	}

	// Print the response body for the GET request
	fmt.Println("Transaction status response body:", string(statusBodyBytes))

	// Unmarshal the status response
	if err := json.Unmarshal(statusBodyBytes, &statusResponse); err != nil {
		t.Fatalf("Failed to decode the status response body: %v", err)
	}

	// Assert that txStatus is "SEEN_ON_NETWORK"
	if statusResponse.TxStatus != "MINED" {
		t.Fatalf("Expected txStatus to be 'MINED', but got '%s'", statusResponse.TxStatus)
	}

	// Print the extracted txStatus (optional, since you're already asserting it)
	fmt.Println("Transaction status:", statusResponse.TxStatus)

}
