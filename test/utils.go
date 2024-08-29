package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	ec "github.com/bitcoin-sv/go-sdk/primitives/ec"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/transaction/template/p2pkh"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bitcoinsv/bsvutil"
	"github.com/stretchr/testify/require"
)

const (
	Status_QUEUED                 = "QUEUED"
	Status_RECEIVED               = "RECEIVED"
	Status_STORED                 = "STORED"
	Status_ANNOUNCED_TO_NETWORK   = "ANNOUNCED_TO_NETWORK"
	Status_REQUESTED_BY_NETWORK   = "REQUESTED_BY_NETWORK"
	Status_SENT_TO_NETWORK        = "SENT_TO_NETWORK"
	Status_ACCEPTED_BY_NETWORK    = "ACCEPTED_BY_NETWORK"
	Status_SEEN_IN_ORPHAN_MEMPOOL = "SEEN_IN_ORPHAN_MEMPOOL"
	Status_SEEN_ON_NETWORK        = "SEEN_ON_NETWORK"
	Status_DOUBLE_SPEND_ATTEMPTED = "DOUBLE_SPEND_ATTEMPTED"
	Status_REJECTED               = "REJECTED"
	Status_MINED                  = "MINED"
)

type TransactionResponseBatch []TransactionResponse

type TransactionRequest struct {
	RawTx string `json:"rawTx"`
}

type TransactionResponse struct {
	BlockHash    *string   `json:"blockHash,omitempty"`
	BlockHeight  *uint64   `json:"blockHeight,omitempty"`
	ExtraInfo    *string   `json:"extraInfo"`
	MerklePath   *string   `json:"merklePath"`
	Status       int       `json:"status"`
	CompetingTxs *[]string `json:"competingTxs"`
	Timestamp    time.Time `json:"timestamp"`
	Title        string    `json:"title"`
	TxStatus     string    `json:"txStatus"`
	Txid         string    `json:"txid"`
}

type ErrorFee struct {
	Detail string `json:"detail"`
	Txid   string `json:"txid"`
}

type NodeUnspentUtxo struct {
	Txid          string  `json:"txid"`
	Vout          uint32  `json:"vout"`
	Address       string  `json:"address"`
	Account       string  `json:"account"`
	ScriptPubKey  string  `json:"scriptPubKey"`
	Amount        float64 `json:"amount"`
	Confirmations int     `json:"confirmations"`
	Spendable     bool    `json:"spendable"`
	Solvable      bool    `json:"solvable"`
	Safe          bool    `json:"safe"`
}

type RawTransaction struct {
	Hex       string `json:"hex"`
	BlockHash string `json:"blockhash,omitempty"`
}

type BlockData struct {
	Height     uint64   `json:"height"`
	Txs        []string `json:"txs"`
	MerkleRoot string   `json:"merkleroot"`
}

func createPayload[T any](t *testing.T, body T) io.Reader {
	payLoad, err := json.Marshal(body)
	require.NoError(t, err)

	return bytes.NewBuffer(payLoad)
}

func getRequest[T any](t *testing.T, url string) T {
	getResp, err := http.Get(url)
	require.NoError(t, err)
	defer getResp.Body.Close()

	var respBody T
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&respBody))

	return respBody
}

func postRequest[T any](t *testing.T, url string, reader io.Reader, headers map[string]string, expectedStatusCode int) T {
	req, err := http.NewRequest("POST", url, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	httpResp, err := client.Do(req)
	require.NoError(t, err)

	defer httpResp.Body.Close()

	if httpResp.StatusCode != expectedStatusCode {
		bodyBytes, err := io.ReadAll(httpResp.Body)
		require.NoError(t, err)
		t.Logf("unexpected status code %d, body: %s", httpResp.StatusCode, string(bodyBytes))
	}

	require.Equal(t, expectedStatusCode, httpResp.StatusCode)

	var response T
	require.NoError(t, json.NewDecoder(httpResp.Body).Decode(&response))

	return response
}

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}

func getNewWalletAddress(t *testing.T) (address, privateKey string) {
	address, err := bitcoind.GetNewAddress()
	require.NoError(t, err)
	t.Logf("new address: %s", address)

	privateKey, err = bitcoind.DumpPrivKey(address)
	require.NoError(t, err)
	t.Logf("new private key: %s", privateKey)

	accountName := "test-account"
	err = bitcoind.SetAccount(address, accountName)
	require.NoError(t, err)

	t.Logf("account %s created", accountName)

	return
}

func sendToAddress(t *testing.T, address string, bsv float64) (txID string) {
	t.Helper()

	txID, err := bitcoind.SendToAddress(address, bsv)
	require.NoError(t, err)

	t.Logf("sent %f to %s: %s", bsv, address, txID)

	return
}

func generate(t *testing.T, amount uint64) string {
	t.Helper()

	// run command instead
	blockHash := execCommandGenerate(t, amount)
	time.Sleep(5 * time.Second)

	t.Logf(
		"generated %d block(s): block hash: %s",
		amount,
		blockHash,
	)

	return blockHash
}

func execCommandGenerate(t *testing.T, amount uint64) string {
	t.Helper()
	t.Logf("Amount to generate: %d", amount)

	hashes, err := bitcoind.Generate(float64(amount))
	require.NoError(t, err)

	return hashes[len(hashes)-1]
}

func getUtxos(t *testing.T, address string) []NodeUnspentUtxo {
	t.Helper()

	data, err := bitcoind.ListUnspent([]string{address})
	require.NoError(t, err)

	result := make([]NodeUnspentUtxo, len(data))

	for index, utxo := range data {
		t.Logf("UTXO Txid: %s, Amount: %f, Address: %s\n", utxo.TXID, utxo.Amount, utxo.Address)
		result[index] = NodeUnspentUtxo{
			Txid:          utxo.TXID,
			Vout:          utxo.Vout,
			Address:       utxo.Address,
			ScriptPubKey:  utxo.ScriptPubKey,
			Amount:        utxo.Amount,
			Confirmations: int(utxo.Confirmations),
		}
	}

	return result
}

func getBlockRootByHeight(t *testing.T, blockHeight int) string {
	t.Helper()
	block, err := bitcoind.GetBlockByHeight(blockHeight)
	require.NoError(t, err)

	return block.MerkleRoot
}

func getRawTx(t *testing.T, txID string) RawTransaction {
	t.Helper()

	rawTx, err := bitcoind.GetRawTransaction(txID)
	require.NoError(t, err)

	return RawTransaction{
		Hex:       rawTx.Hex,
		BlockHash: rawTx.BlockHash,
	}
}

func getBlockDataByBlockHash(t *testing.T, blockHash string) BlockData {
	t.Helper()

	block, err := bitcoind.GetBlock(blockHash)
	require.NoError(t, err)

	return BlockData{
		Height:     block.Height,
		Txs:        block.Tx,
		MerkleRoot: block.MerkleRoot,
	}
}

func createTx(privateKey string, address string, utxo NodeUnspentUtxo, fee ...uint64) (*sdkTx.Transaction, error) {
	return createTxFrom(privateKey, address, []NodeUnspentUtxo{utxo}, fee...)
}

func createTxFrom(privateKey string, address string, utxos []NodeUnspentUtxo, fee ...uint64) (*sdkTx.Transaction, error) {
	tx := sdkTx.NewTransaction()

	// Add an input using the UTXOs
	for _, utxo := range utxos {
		utxoTxID := utxo.Txid
		utxoVout := utxo.Vout
		utxoSatoshis := uint64(utxo.Amount * 1e8) // Convert BTC to satoshis
		utxoScript := utxo.ScriptPubKey

		u, err := sdkTx.NewUTXO(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
		if err != nil {
			return nil, fmt.Errorf("failed creating UTXO: %v", err)
		}
		err = tx.AddInputsFromUTXOs(u)
		if err != nil {
			return nil, fmt.Errorf("failed adding input: %v", err)
		}
	}
	// Add an output to the address you've previously created
	recipientAddress := address

	var feeValue uint64
	if len(fee) > 0 {
		feeValue = fee[0]
	} else {
		feeValue = 20 // Set your default fee value here
	}
	amountToSend := tx.TotalInputSatoshis() - feeValue

	err := tx.PayToAddress(recipientAddress, amountToSend)
	if err != nil {
		return nil, fmt.Errorf("failed to pay to address: %v", err)
	}

	// Sign the input
	wif, err := bsvutil.DecodeWIF(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode WIF: %v", err)
	}

	// Extract raw private key bytes directly from the WIF structure
	privateKeyDecoded := wif.PrivKey.Serialize()
	pk, _ := ec.PrivateKeyFromBytes(privateKeyDecoded)

	unlockingScriptTemplate, err := p2pkh.Unlock(pk, nil)
	if err != nil {
		return nil, err
	}

	for _, input := range tx.Inputs {
		input.UnlockingScriptTemplate = unlockingScriptTemplate
	}

	err = tx.Sign()
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func generateRandomString(length int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

type callbackResponseFn func(w http.ResponseWriter, rc chan *TransactionResponse, ec chan error, status *TransactionResponse)

// use buffered channels for multiple callbacks
func startCallbackSrv(t *testing.T, receivedChan chan *TransactionResponse, errChan chan error, alternativeResponseFn callbackResponseFn) (callbackUrl, token string, shutdownFn func()) {
	t.Helper()
	callback := generateRandomString(16)
	token = "1234"
	expectedAuthHeader := fmt.Sprintf("Bearer %s", token)

	hostname, err := os.Hostname()
	require.NoError(t, err)

	callbackUrl = fmt.Sprintf("http://%s:9000/%s", hostname, callback)

	readPayload := func(req *http.Request) (*TransactionResponse, error) {
		defer func() {
			err := req.Body.Close()
			if err != nil {
				t.Log("failed to close body")
			}
		}()

		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		var status TransactionResponse
		err = json.Unmarshal(bodyBytes, &status)
		if err != nil {
			return nil, err
		}

		return &status, nil
	}

	http.HandleFunc(fmt.Sprintf("/%s", callback), func(w http.ResponseWriter, req *http.Request) {
		// check auth
		if expectedAuthHeader != req.Header.Get("Authorization") {
			errChan <- fmt.Errorf("auth header %s not as expected %s", expectedAuthHeader, req.Header.Get("Authorization"))
			err = respondToCallback(w, false)
			if err != nil {
				t.Fatalf("Failed to respond to callback: %v", err)
			}
			return
		}

		status, err := readPayload(req)
		if err != nil {
			errChan <- fmt.Errorf("read callback payload failed: %v", err)
			return
		}

		if alternativeResponseFn != nil {
			alternativeResponseFn(w, receivedChan, errChan, status)
		} else {
			t.Log("callback received, responding success")
			err = respondToCallback(w, true)
			if err != nil {
				t.Fatalf("Failed to respond to callback: %v", err)
			}

			receivedChan <- status
		}
	})

	srv := &http.Server{Addr: ":9000"}
	shutdownFn = func() {
		t.Logf("shutting down callback listener %s", callbackUrl)
		close(receivedChan)
		close(errChan)

		if err := srv.Shutdown(context.TODO()); err != nil {
			t.Fatal("failed to shut down server")
		}
		t.Log("callback listener is down")
	}

	go func(server *http.Server) {
		t.Logf("starting callback server %s", callbackUrl)
		err := server.ListenAndServe()
		if err != nil {
			return
		}
	}(srv)

	return
}

func respondToCallback(w http.ResponseWriter, success bool) error {
	resp := make(map[string]string)
	if success {
		resp["message"] = "Success"
		w.WriteHeader(http.StatusOK)
	} else {
		resp["message"] = "Bad Request"
		w.WriteHeader(http.StatusBadRequest)
	}

	jsonResp, _ := json.Marshal(resp)
	_, err := w.Write(jsonResp)
	if err != nil {
		return err
	}
	return nil
}

func fundNewWallet(t *testing.T) (addr, privKey string) {
	t.Helper()

	addr, privKey = getNewWalletAddress(t)
	sendToAddress(t, addr, 0.001)
	// mine a block with the transaction from above
	generate(t, 1)

	return
}

func testTxSubmission(t *testing.T, callbackUrl string, token string, tx *sdkTx.Transaction) {
	rawTx, err := tx.EFHex()
	require.NoError(t, err)

	response := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}),
		map[string]string{
			"X-WaitFor":       Status_SEEN_ON_NETWORK,
			"X-CallbackUrl":   callbackUrl,
			"X-CallbackToken": token,
		}, http.StatusOK)
	require.Equal(t, Status_SEEN_ON_NETWORK, response.TxStatus)
}

func prepareCallback(t *testing.T, callbackNumbers int) (chan *TransactionResponse, chan error, callbackResponseFn) {
	callbackReceivedChan := make(chan *TransactionResponse, 100) // do not block callback server responses
	callbackErrChan := make(chan error, 100)

	responseVisitMap := make(map[string]int)
	mu := &sync.Mutex{}

	calbackResponseFn := func(w http.ResponseWriter, rc chan *TransactionResponse, ec chan error, status *TransactionResponse) {
		mu.Lock()
		callbackNumber := responseVisitMap[status.Txid]
		callbackNumber++
		responseVisitMap[status.Txid] = callbackNumber
		mu.Unlock()
		// Let ARC send the same callback few times. Respond with success on the last one.
		respondWithSuccess := false
		if callbackNumber < callbackNumbers {
			respondWithSuccess = false

		} else {
			respondWithSuccess = true
		}

		err := respondToCallback(w, respondWithSuccess)
		if err != nil {
			t.Fatalf("Failed to respond to callback: %v", err)
		}

		rc <- status
	}
	return callbackReceivedChan, callbackErrChan, calbackResponseFn
}
