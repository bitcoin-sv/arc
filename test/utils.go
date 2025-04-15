//go:build e2e

package test

import (
	"bytes"
	cRand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	"github.com/ccoveille/go-safecast"
	safe "github.com/ccoveille/go-safecast"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
)

const (
	StatusQueued               = "QUEUED"
	StatusSeenInOrphanMempool  = "SEEN_IN_ORPHAN_MEMPOOL"
	StatusSeenOnNetwork        = "SEEN_ON_NETWORK"
	StatusDoubleSpendAttempted = "DOUBLE_SPEND_ATTEMPTED"
	StatusRejected             = "REJECTED"
	StatusMined                = "MINED"
	StatusMinedInStaleBlock    = "MINED_IN_STALE_BLOCK"
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

func (c TransactionResponse) GetTxID() string {
	return c.Txid
}

type CallbackBatchResponse struct {
	Count     int                    `json:"count"`
	Callbacks []*TransactionResponse `json:"callbacks,omitempty"`
}

func (c CallbackBatchResponse) GetTxID() string {
	return c.Callbacks[0].Txid
}

type Response interface {
	GetTxID() string
}

type ErrorFee struct {
	Detail string `json:"detail"`
	Txid   string `json:"txid"`
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
	t.Helper()

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

// registerHandlerForCallback registers a new handler function that responds to callbacks with bad request response first and at second try with success or alternative given response function. It returns the callback URL and token to be used.
func registerHandlerForCallback[T any](t *testing.T, receivedChan chan T, errChan chan error, alternativeResponseFn func(w http.ResponseWriter, rc chan T, ec chan error, status T), mux *http.ServeMux) (callbackURL, token string) {
	t.Helper()

	b := make([]byte, 16)
	_, err := cRand.Read(b)
	require.NoError(t, err)
	callback := hex.EncodeToString(b)

	token = "1234"
	expectedAuthHeader := fmt.Sprintf("Bearer %s", token)

	hostname, err := os.Hostname()
	require.NoError(t, err)

	callbackURL = fmt.Sprintf("http://%s:9000/%s", hostname, callback)
	t.Logf("random callback URL: %s", callbackURL)

	mux.HandleFunc(fmt.Sprintf("/%s", callback), func(w http.ResponseWriter, req *http.Request) {
		// check auth
		if expectedAuthHeader != req.Header.Get("Authorization") {
			errChan <- fmt.Errorf("auth header %s not as expected %s", expectedAuthHeader, req.Header.Get("Authorization"))
			err = respondToCallback(w, false)
			if err != nil {
				t.Fatalf("Failed to respond to callback: %v", err)
			}
			return
		}

		status, err := readPayload[T](t, req)
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

	return callbackURL, token
}

func readPayload[T any](t *testing.T, req *http.Request) (T, error) {
	var res T

	defer func() {
		err := req.Body.Close()
		if err != nil {
			t.Log("failed to close body")
		}
	}()

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return res, err
	}

	err = json.Unmarshal(bodyBytes, &res)
	if err != nil {
		return res, err
	}

	return res, nil
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

func testTxSubmission(t *testing.T, callbackURL string, token string, callbackBatch bool, tx *sdkTx.Transaction) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	rawTx, err := tx.EFHex()
	require.NoError(t, err)

	response := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx}),
		map[string]string{
			"X-WaitFor":       StatusSeenOnNetwork,
			"X-CallbackUrl":   callbackURL,
			"X-CallbackToken": token,
			"X-CallbackBatch": strconv.FormatBool(callbackBatch),
			"X-MaxTimeout":    "7",
		}, http.StatusOK)
	require.Equal(t, StatusSeenOnNetwork, response.TxStatus)
}

func getResponseFunc[T Response](t *testing.T, respondSuccessAtCallbacks int) func(w http.ResponseWriter, rc chan T, ec chan error, status T) {
	t.Helper()

	responseVisitMap := make(map[string]int)
	mu := &sync.Mutex{}

	calbackResponseFn := func(w http.ResponseWriter, rc chan T, _ chan error, status T) {
		mu.Lock()
		txID := status.GetTxID()
		callbackCounter := responseVisitMap[txID]
		callbackCounter++
		responseVisitMap[txID] = callbackCounter
		mu.Unlock()

		// Let ARC send the same callback few times
		respondWithSuccess := callbackCounter >= respondSuccessAtCallbacks

		err := respondToCallback(w, respondWithSuccess)
		if err != nil {
			t.Fatalf("Failed to respond to callback: %v", err)
		}

		rc <- status
	}
	return calbackResponseFn
}

func generateNewUnlockingScriptFromRandomKey() (*script.Script, error) {
	privKey, err := ec.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	address, err := script.NewAddressFromPublicKey(privKey.PubKey(), false)
	if err != nil {
		return nil, err
	}

	sc, err := p2pkh.Lock(address)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func checkMerklePath(t *testing.T, statusResponse TransactionResponse) {
	require.NotNil(t, statusResponse.MerklePath)

	bump, err := bc.NewBUMPFromStr(*statusResponse.MerklePath)
	require.NoError(t, err)

	jsonB, err := json.Marshal(bump)
	require.NoError(t, err)
	t.Logf("BUMPjson: %s", string(jsonB))

	root, err := bump.CalculateRootGivenTxid(statusResponse.Txid)
	require.NoError(t, err)

	require.NotNil(t, statusResponse.BlockHeight)
	bh, err := safecast.ToInt(*statusResponse.BlockHeight)
	require.NoError(t, err)
	blockRoot := node_client.GetBlockRootByHeight(t, bitcoind, bh)
	require.Equal(t, blockRoot, root)
}

func checkStatus(t *testing.T, txID string, expectedStatus string) {
	statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, txID)
	statusResp := getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, expectedStatus, statusResp.TxStatus)
}

func checkStatusBlockHash(t *testing.T, txID string, expectedStatus string, expectedBlockHash string) {
	statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, txID)
	statusResp := getRequest[TransactionResponse](t, statusURL)
	require.Equal(t, expectedStatus, statusResp.TxStatus)
	require.Equal(t, expectedBlockHash, *statusResp.BlockHash)
}

func getMerklePath(t *testing.T, txID string) string {
	rawTx, _ := bitcoind.GetRawTransaction(txID)
	blockData := node_client.GetBlockDataByBlockHash(t, bitcoind, rawTx.BlockHash)
	blockTxHashes := make([]*chainhash.Hash, len(blockData.Txs))
	var txIndex uint64

	for i, blockTx := range blockData.Txs {
		h, err := chainhash.NewHashFromStr(blockTx)
		require.NoError(t, err)

		blockTxHashes[i] = h

		if blockTx == rawTx.Hash {
			ind, err := safe.ToUint64(i)
			require.NoError(t, err)
			txIndex = ind
		}
	}

	merkleTree := bc.BuildMerkleTreeStoreChainHash(blockTxHashes)
	require.Equal(t, merkleTree[len(merkleTree)-1].String(), blockData.MerkleRoot)

	merklePath, err := bc.NewBUMPFromMerkleTreeAndIndex(blockData.Height, merkleTree, txIndex)
	require.NoError(t, err)
	merklePathStr, err := merklePath.String()
	require.NoError(t, err)

	return merklePathStr
}
