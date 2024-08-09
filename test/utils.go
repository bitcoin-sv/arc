package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoinsv/bsvd/bsvec"
	"github.com/bitcoinsv/bsvutil"
	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
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
	MerklePath  string      `json:"merklePath"`
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

type TransactionRequest struct {
	RawTx string `json:"rawTx"`
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

func getRequest[T any](t *testing.T, url string) T {
	getResp, err := http.Get(url)
	require.NoError(t, err)
	defer getResp.Body.Close()

	var respBody T
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&respBody))

	return respBody
}

func postRequest[T any](t *testing.T, url string, reader io.Reader) T {
	req, err := http.NewRequest("POST", url, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	httpResp, err := client.Do(req)
	require.NoError(t, err)

	defer httpResp.Body.Close()
	require.Equal(t, http.StatusOK, httpResp.StatusCode)

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

func createTx(privateKey string, address string, utxo NodeUnspentUtxo, fee ...uint64) (*bt.Tx, error) {
	tx := bt.NewTx()

	// Add an input using the first UTXO
	utxoTxID := utxo.Txid
	utxoVout := utxo.Vout
	utxoSatoshis := uint64(utxo.Amount * 1e8) // Convert BTC to satoshis
	utxoScript := utxo.ScriptPubKey

	err := tx.From(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
	if err != nil {
		return nil, fmt.Errorf("failed adding input: %v", err)
	}

	// Add an output to the address you've previously created
	recipientAddress := address

	var feeValue uint64
	if len(fee) > 0 {
		feeValue = fee[0]
	} else {
		feeValue = 20 // Set your default fee value here
	}
	amountToSend := utxoSatoshis - feeValue

	recipientScript, err := bscript.NewP2PKHFromAddress(recipientAddress)
	if err != nil {
		return nil, fmt.Errorf("failed converting address to script: %v", err)
	}

	err = tx.PayTo(recipientScript, amountToSend)
	if err != nil {
		return nil, fmt.Errorf("failed adding output: %v", err)
	}

	// Sign the input

	wif, err := bsvutil.DecodeWIF(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode WIF: %v", err)
	}

	// Extract raw private key bytes directly from the WIF structure
	privateKeyDecoded := wif.PrivKey.Serialize()

	pk, _ := bec.PrivKeyFromBytes(bsvec.S256(), privateKeyDecoded)
	unlockerGetter := unlocker.Getter{PrivateKey: pk}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return nil, fmt.Errorf("sign failed: %v", err)
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

type callbackResponseFn func(w http.ResponseWriter, rc chan *api.TransactionStatus, ec chan error, status *api.TransactionStatus)

// use buffered channels for multiple callbacks
func startCallbackSrv(t *testing.T, receivedChan chan *api.TransactionStatus, errChan chan error,
	alternativeResponseFn callbackResponseFn) (
	callbackUrl, token string, shutdownFn func(),
) {
	t.Helper()
	callback := generateRandomString(16)
	token = "1234"
	expectedAuthHeader := fmt.Sprintf("Bearer %s", token)

	hostname, err := os.Hostname()
	require.NoError(t, err)

	callbackUrl = fmt.Sprintf("http://%s:9000/%s", hostname, callback)

	readPayload := func(req *http.Request) (*api.TransactionStatus, error) {
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

		var status api.TransactionStatus
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
		t.Log("shutting down callback listener")
		close(receivedChan)
		close(errChan)

		if err := srv.Shutdown(context.TODO()); err != nil {
			t.Fatal("failed to shut down server")
		}
		t.Log("callback listener is down")
	}

	go func(server *http.Server) {
		t.Log("starting callback server")
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
