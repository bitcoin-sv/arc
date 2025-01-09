//go:build e2e

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	ec "github.com/bitcoin-sv/go-sdk/primitives/ec"
	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/transaction/template/p2pkh"
	"github.com/stretchr/testify/require"
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

type CallbackBatchResponse struct {
	Count     int                    `json:"count"`
	Callbacks []*TransactionResponse `json:"callbacks,omitempty"`
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

func generateRandomString(length int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

type (
	callbackResponseFn      func(w http.ResponseWriter, rc chan *TransactionResponse, ec chan error, status *TransactionResponse)
	callbackBatchResponseFn func(w http.ResponseWriter, rc chan *CallbackBatchResponse, ec chan error, status *CallbackBatchResponse)
)

// use buffered channels for multiple callbacks
func startCallbackSrv(t *testing.T, receivedChan chan *TransactionResponse, errChan chan error, alternativeResponseFn callbackResponseFn) (callbackURL, token string, shutdownFn func()) {
	t.Helper()

	callback := generateRandomString(16)
	token = "1234"
	expectedAuthHeader := fmt.Sprintf("Bearer %s", token)

	hostname, err := os.Hostname()
	require.NoError(t, err)

	callbackURL = fmt.Sprintf("http://%s:9000/%s", hostname, callback)

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

		status, err := readPayload[*TransactionResponse](t, req)
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
		t.Logf("shutting down callback listener %s", callbackURL)

		if err := srv.Shutdown(context.TODO()); err != nil {
			t.Fatal("failed to shut down server")
		}
		t.Log("callback listener is down")

		close(receivedChan)
		close(errChan)
	}

	go func(server *http.Server) {
		t.Logf("starting callback server %s", callbackURL)
		err := server.ListenAndServe()
		if err != nil {
			return
		}
	}(srv)

	return
}

// use buffered channels for multiple callbacks
func startBatchCallbackSrv(t *testing.T, receivedChan chan *CallbackBatchResponse, errChan chan error, alternativeResponseFn callbackBatchResponseFn) (callbackURL, token string, shutdownFn func()) {
	t.Helper()

	callback := generateRandomString(16)
	token = "1234"
	expectedAuthHeader := fmt.Sprintf("Bearer %s", token)

	hostname, err := os.Hostname()
	require.NoError(t, err)

	callbackURL = fmt.Sprintf("http://%s:9000/%s/batch", hostname, callback)

	http.HandleFunc(fmt.Sprintf("/%s/batch", callback), func(w http.ResponseWriter, req *http.Request) {
		// check auth
		if expectedAuthHeader != req.Header.Get("Authorization") {
			errChan <- fmt.Errorf("auth header %s not as expected %s", expectedAuthHeader, req.Header.Get("Authorization"))
			err = respondToCallback(w, false)
			if err != nil {
				t.Fatalf("Failed to respond to callback: %v", err)
			}
			return
		}

		status, err := readPayload[*CallbackBatchResponse](t, req)
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
		t.Logf("shutting down callback listener %s", callbackURL)
		close(receivedChan)
		close(errChan)

		if err := srv.Shutdown(context.TODO()); err != nil {
			t.Fatal("failed to shut down server")
		}
		t.Log("callback listener is down")
	}

	go func(server *http.Server) {
		t.Logf("starting callback server %s", callbackURL)
		err := server.ListenAndServe()
		if err != nil {
			return
		}
	}(srv)

	return
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

func prepareCallback(t *testing.T, callbackNumbers int) (chan *TransactionResponse, chan error, callbackResponseFn) {
	t.Helper()

	callbackReceivedChan := make(chan *TransactionResponse, 100) // do not block callback server responses
	callbackErrChan := make(chan error, 100)

	responseVisitMap := make(map[string]int)
	mu := &sync.Mutex{}

	calbackResponseFn := func(w http.ResponseWriter, rc chan *TransactionResponse, _ chan error, status *TransactionResponse) {
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

func prepareBatchCallback(t *testing.T, callbackNumbers int) (chan *CallbackBatchResponse, chan error, callbackBatchResponseFn) {
	t.Helper()

	callbackReceivedChan := make(chan *CallbackBatchResponse, 100) // do not block callback server responses
	callbackErrChan := make(chan error, 100)

	responseVisitMap := make(map[string]int)
	mu := &sync.Mutex{}

	calbackResponseFn := func(w http.ResponseWriter, rc chan *CallbackBatchResponse, _ chan error, status *CallbackBatchResponse) {
		mu.Lock()
		callbackNumber := responseVisitMap[status.Callbacks[0].Txid]
		callbackNumber++
		responseVisitMap[status.Callbacks[0].Txid] = callbackNumber
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
