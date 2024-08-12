package test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
	"github.com/stretchr/testify/require"
)

func TestSubmitSingle(t *testing.T) {
	address, privateKey := fundNewWallet(t)

	utxos := getUtxos(t, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	tx, err := createTx(privateKey, address, utxos[0])
	require.NoError(t, err)

	malFormedRawTx, err := os.ReadFile("./fixtures/malformedTxHexString.txt")
	require.NoError(t, err)

	type malformedTransactionRequest struct {
		Transaction string `json:"transaction"`
	}

	tt := []struct {
		name string
		body any

		expectedStatusCode int
	}{
		{
			name: "post single - success",
			body: TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())},

			expectedStatusCode: http.StatusOK,
		},
		{
			name: "post single - malformed tx",
			body: TransactionRequest{RawTx: hex.EncodeToString(malFormedRawTx)},

			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "post single - wrong payload",
			body: malformedTransactionRequest{Transaction: "fake data"},

			expectedStatusCode: http.StatusBadRequest,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			// Send POST request
			response := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, tc.body), nil, tc.expectedStatusCode)

			if tc.expectedStatusCode != http.StatusOK {
				return
			}
			require.Equal(t, Status_SEEN_ON_NETWORK, response.TxStatus)

			time.Sleep(1 * time.Second) // give ARC time to perform the status update on DB

			// repeat request to ensure response remains the same
			txID := response.Txid
			response = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}), nil, http.StatusOK)
			require.Equal(t, txID, response.Txid)
			require.Equal(t, Status_SEEN_ON_NETWORK, response.TxStatus)

			// Check transaction status
			statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, txID)
			statusResponse := getRequest[TransactionResponse](t, fmt.Sprintf("%s/%s", arcEndpointV1Tx, txID))
			require.Equal(t, Status_SEEN_ON_NETWORK, statusResponse.TxStatus)

			t.Logf("Transaction status: %s", statusResponse.TxStatus)

			generate(t, 10)

			statusResponse = getRequest[TransactionResponse](t, statusUrl)
			require.Equal(t, Status_MINED, statusResponse.TxStatus)

			t.Logf("Transaction status: %s", statusResponse.TxStatus)

			// Check Merkle path
			require.NotNil(t, statusResponse.MerklePath)
			t.Logf("BUMP: %s", *statusResponse.MerklePath)
			bump, err := bc.NewBUMPFromStr(*statusResponse.MerklePath)
			require.NoError(t, err)

			jsonB, err := json.Marshal(bump)
			require.NoError(t, err)
			t.Logf("BUMPjson: %s", string(jsonB))

			root, err := bump.CalculateRootGivenTxid(tx.TxID())
			require.NoError(t, err)

			require.NotNil(t, statusResponse.BlockHeight)
			blockRoot := getBlockRootByHeight(t, int(*statusResponse.BlockHeight))
			require.Equal(t, blockRoot, root)
		})
	}
}

func TestSubmitMined(t *testing.T) {
	t.Run("submit mined tx", func(t *testing.T) {

		// submit an unregistered, already mined transaction. ARC should return the status as MINED for the transaction.

		// given
		address, _ := fundNewWallet(t)
		utxos := getUtxos(t, address)

		rawTx, _ := bitcoind.GetRawTransaction(utxos[0].Txid)
		tx, _ := bt.NewTxFromString(rawTx.Hex)

		callbackReceivedChan := make(chan *TransactionResponse)
		callbackErrChan := make(chan error)

		callbackUrl, token, shutdown := startCallbackSrv(t, callbackReceivedChan, callbackErrChan, nil)
		defer shutdown()

		// when
		_ = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}),
			map[string]string{
				"X-WaitFor":       Status_MINED,
				"X-CallbackUrl":   callbackUrl,
				"X-CallbackToken": token,
			}, http.StatusOK)

		// wait for callback
		callbackTimeout := time.After(10 * time.Second)

		select {
		case status := <-callbackReceivedChan:
			require.Equal(t, rawTx.TxID, status.Txid)
			require.Equal(t, Status_MINED, status.TxStatus)
		case err := <-callbackErrChan:
			t.Fatalf("callback error: %v", err)
		case <-callbackTimeout:
			t.Fatal("callback exceeded timeout")
		}
	})
}

func TestSubmitQueued(t *testing.T) {
	t.Run("queued", func(t *testing.T) {
		address, privateKey := getNewWalletAddress(t)
		sendToAddress(t, address, 0.001)

		hash := generate(t, 1)
		t.Logf("generated 1 block: %s", hash)

		utxos := getUtxos(t, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		tx, err := createTx(privateKey, address, utxos[0])
		require.NoError(t, err)

		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}),
			map[string]string{
				"X-WaitFor":    Status_QUEUED,
				"X-MaxTimeout": strconv.Itoa(1),
			}, http.StatusOK)

		require.Equal(t, Status_QUEUED, resp.TxStatus)

		statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx.TxID())
	checkSeenLoop:
		for {
			select {
			case <-time.NewTicker(1 * time.Second).C:

				statusResponse := getRequest[TransactionResponse](t, statusUrl)
				if statusResponse.TxStatus == Status_SEEN_ON_NETWORK {
					break checkSeenLoop
				}
			case <-time.NewTimer(10 * time.Second).C:
				t.Fatal("transaction not seen on network after 10s")
			}
		}

		generate(t, 10)

	checkMinedLoop:
		for {
			select {
			case <-time.NewTicker(1 * time.Second).C:
				statusResponse := getRequest[TransactionResponse](t, statusUrl)
				if statusResponse.TxStatus == Status_MINED {
					break checkMinedLoop
				}

			case <-time.NewTimer(15 * time.Second).C:
				t.Fatal("transaction not mined after 15s")
			}
		}
	})
}

func TestCallback(t *testing.T) {

	t.Run("post transaction with callback url and token", func(t *testing.T) {
		address, privateKey := fundNewWallet(t)

		utxos := getUtxos(t, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		tx, err := createTx(privateKey, address, utxos[0])
		require.NoError(t, err)

		const callbackNumbers = 2                                                // cannot be greater than 5
		callbackReceivedChan := make(chan *TransactionResponse, callbackNumbers) // do not block callback server responses
		callbackErrChan := make(chan error, callbackNumbers)
		callbackIteration := 0

		calbackResponseFn := func(w http.ResponseWriter, rc chan *TransactionResponse, ec chan error, status *TransactionResponse) {
			callbackIteration++

			// Let ARC send the callback few times. Respond with success on the last one.
			respondWithSuccess := false
			if callbackIteration < callbackNumbers {
				t.Logf("%d callback received, responding bad request", callbackIteration)
				respondWithSuccess = false

			} else {
				t.Logf("%d callback received, responding success", callbackIteration)
				respondWithSuccess = true
			}

			err = respondToCallback(w, respondWithSuccess)
			if err != nil {
				t.Fatalf("Failed to respond to callback: %v", err)
			}

			rc <- status
		}
		callbackUrl, token, shutdown := startCallbackSrv(t, callbackReceivedChan, callbackErrChan, calbackResponseFn)
		defer shutdown()

		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}),
			map[string]string{
				"X-WaitFor":       Status_SEEN_ON_NETWORK,
				"X-CallbackUrl":   callbackUrl,
				"X-CallbackToken": token,
			}, http.StatusOK)

		require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)

		generate(t, 10)

		statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx.TxID())
		statusResp := getRequest[TransactionResponse](t, statusUrl)

		require.NotNil(t, statusResp.MerklePath)
		_, err = bc.NewBUMPFromStr(*statusResp.MerklePath)
		require.NoError(t, err)

		for i := 0; i < callbackNumbers; i++ {
			t.Logf("callback iteration %d", i)
			callbackTimeout := time.After(time.Second * time.Duration(i) * 2 * 5)

			select {
			case callback := <-callbackReceivedChan:
				require.NotNil(t, callback)

				t.Logf("Callback %d result: %s", i, callback.TxStatus)
				require.Equal(t, statusResp.Txid, callback.Txid)
				require.Equal(t, *statusResp.BlockHeight, *callback.BlockHeight)
				require.Equal(t, *statusResp.BlockHash, *callback.BlockHash)
				require.Equal(t, Status_MINED, callback.TxStatus)

			case err := <-callbackErrChan:
				t.Fatalf("callback received - failed to parse callback %v", err)
			case <-callbackTimeout:
				t.Fatal("callback not received - timeout")
			}
		}
	})
}

func TestSkipValidation(t *testing.T) {
	tt := []struct {
		name              string
		skipFeeValidation bool
		skipTxValidation  bool
	}{
		{
			name:              "post transaction with skip fee",
			skipFeeValidation: true,
			skipTxValidation:  false,
		},
		{
			name:              "post transaction with skip script validation",
			skipFeeValidation: false,
			skipTxValidation:  true,
		},
		{
			name:              "post tx without validation",
			skipFeeValidation: true,
			skipTxValidation:  true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			address, privateKey := fundNewWallet(t)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			fee := uint64(0)

			tx, err := createTx(privateKey, address, utxos[0], fee)
			require.NoError(t, err)

			fmt.Println("Transaction with Zero fee:", tx)

			resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}),
				map[string]string{
					"X-WaitFor":           Status_SEEN_ON_NETWORK,
					"X-SkipFeeValidation": strconv.FormatBool(tc.skipFeeValidation),
					"X-SkipTxValidation":  strconv.FormatBool(tc.skipTxValidation),
				}, http.StatusOK)

			require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)
		})
	}
}
