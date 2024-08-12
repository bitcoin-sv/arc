package test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
	"github.com/stretchr/testify/require"
)

func TestPostCallbackToken(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "post transaction with callback url and token",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			address, privateKey := fundNewWallet(t)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			tx, err := createTx(privateKey, address, utxos[0])
			require.NoError(t, err)

			arcClient, err := api.NewClientWithResponses(arcEndpoint)
			require.NoError(t, err)

			ctx := context.Background()

			const callbackNumbers = 2                                                  // cannot be greater than 5
			callbackReceivedChan := make(chan *api.TransactionStatus, callbackNumbers) // do not block callback server responses
			callbackErrChan := make(chan error, callbackNumbers)
			callbackIteration := 0

			calbackResponseFn := func(w http.ResponseWriter, rc chan *api.TransactionStatus, ec chan error, status *api.TransactionStatus) {
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

			waitFor := Status_SEEN_ON_NETWORK
			params := &api.POSTTransactionParams{
				XWaitFor:       &waitFor,
				XCallbackUrl:   &callbackUrl,
				XCallbackToken: &token,
			}

			arcBody := api.POSTTransactionJSONRequestBody{
				RawTx: hex.EncodeToString(tx.ExtendedBytes()),
			}

			var response *api.POSTTransactionResponse
			response, err = arcClient.POSTTransactionWithResponse(ctx, params, arcBody)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, response.StatusCode())
			require.NotNil(t, response.JSON200)
			require.Equal(t, Status_SEEN_ON_NETWORK, response.JSON200.TxStatus)

			generate(t, 10)

			var statusResponse *api.GETTransactionStatusResponse
			statusResponse, err = arcClient.GETTransactionStatusWithResponse(ctx, response.JSON200.Txid)
			require.NoError(t, err)
			require.NotNil(t, statusResponse)
			require.NotNil(t, statusResponse.JSON200)
			t.Logf("GETTransactionStatusWithResponse result: %s", *statusResponse.JSON200.TxStatus)

			require.NotNil(t, statusResponse.JSON200.MerklePath)
			_, err = bc.NewBUMPFromStr(*statusResponse.JSON200.MerklePath)
			require.NoError(t, err)

			for i := 0; i < callbackNumbers; i++ {
				t.Logf("callback iteration %d", i)
				callbackTimeout := time.After(time.Second * time.Duration(i) * 2 * 5)

				select {
				case callback := <-callbackReceivedChan:
					require.NotNil(t, callback)

					t.Logf("Callback %d result: %s", i, *callback.TxStatus)
					require.Equal(t, statusResponse.JSON200.Txid, callback.Txid)
					require.Equal(t, *statusResponse.JSON200.BlockHeight, *callback.BlockHeight)
					require.Equal(t, *statusResponse.JSON200.BlockHash, *callback.BlockHash)
					require.Equal(t, Status_MINED, *callback.TxStatus)

				case err := <-callbackErrChan:
					t.Fatalf("callback received - failed to parse callback %v", err)
				case <-callbackTimeout:
					t.Fatal("callback not received - timeout")
				}
			}
		})
	}
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

			request := TransactionRequest{
				RawTx: hex.EncodeToString(tx.ExtendedBytes()),
			}

			resp := postRequest[TransactionResponse](t,
				arcEndpointV1Tx,
				createPayload(t, request),
				map[string]string{
					"X-WaitFor":           Status_SEEN_ON_NETWORK,
					"X-SkipFeeValidation": strconv.FormatBool(tc.skipFeeValidation),
					"X-SkipTxValidation":  strconv.FormatBool(tc.skipTxValidation),
				},
			)

			require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)
		})
	}
}

func Test_E2E_Success(t *testing.T) {
	tx := createTxHexStringExtended(t)
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, hex.EncodeToString(tx.ExtendedBytes()))

	// Send POST request
	response := postRequest[Response](t, arcEndpointV1Tx, strings.NewReader(jsonPayload), nil)
	txID := response.Txid
	require.Equal(t, Status_SEEN_ON_NETWORK, response.TxStatus)

	time.Sleep(1 * time.Second) // give ARC time to perform the status update on DB

	// repeat request to ensure response remains the same
	response = postRequest[Response](t, arcEndpointV1Tx, strings.NewReader(jsonPayload), nil)
	require.Equal(t, txID, response.Txid)
	require.Equal(t, Status_SEEN_ON_NETWORK, response.TxStatus)

	// Check transaction status
	statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, txID)
	statusResponse := getRequest[TxStatusResponse](t, statusUrl)
	require.Equal(t, Status_SEEN_ON_NETWORK, statusResponse.TxStatus)

	t.Logf("Transaction status: %s", statusResponse.TxStatus)

	generate(t, 10)

	statusResponse = getRequest[TxStatusResponse](t, statusUrl)
	require.Equal(t, Status_MINED, statusResponse.TxStatus)

	t.Logf("Transaction status: %s", statusResponse.TxStatus)

	// Check Merkle path
	t.Logf("BUMP: %s", statusResponse.MerklePath)

	bump, err := bc.NewBUMPFromStr(statusResponse.MerklePath)
	require.NoError(t, err)

	jsonB, err := json.Marshal(bump)
	require.NoError(t, err)
	t.Logf("BUMPjson: %s", string(jsonB))

	root, err := bump.CalculateRootGivenTxid(tx.TxID())
	require.NoError(t, err)

	blockRoot := getBlockRootByHeight(t, statusResponse.BlockHeight)
	require.Equal(t, blockRoot, root)
}

func TestPostTx_Queued(t *testing.T) {
	t.Run("queued", func(t *testing.T) {
		address, privateKey := getNewWalletAddress(t)
		sendToAddress(t, address, 0.001)

		hash := generate(t, 1)
		t.Logf("generated 1 block: %s", hash)

		utxos := getUtxos(t, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		tx, err := createTx(privateKey, address, utxos[0])
		require.NoError(t, err)

		arcClient, err := api.NewClientWithResponses(arcEndpoint)
		require.NoError(t, err)

		rawTxString := hex.EncodeToString(tx.ExtendedBytes())
		body := api.POSTTransactionJSONRequestBody{
			RawTx: rawTxString,
		}

		ctx := context.Background()

		expectedStatus := string(Status_QUEUED)
		params := &api.POSTTransactionParams{
			XWaitFor:    &expectedStatus,
			XMaxTimeout: PtrTo(1),
		}
		response, err := arcClient.POSTTransactionWithResponse(ctx, params, body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, response.StatusCode())
		require.NotNil(t, response.JSON200)
		require.Equalf(t, expectedStatus, response.JSON200.TxStatus, "status of response: %s does not match expected status: %s for tx ID %s", response.JSON200.TxStatus, expectedStatus, tx.TxID())

	checkSeenLoop:
		for {
			select {
			case <-time.NewTicker(1 * time.Second).C:
				statusResponse, err := arcClient.GETTransactionStatusWithResponse(ctx, tx.TxID())
				require.NoError(t, err)

				if statusResponse != nil && statusResponse.JSON200 != nil && statusResponse.JSON200.TxStatus != nil && Status_SEEN_ON_NETWORK == *statusResponse.JSON200.TxStatus {
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
				statusResponse, err := arcClient.GETTransactionStatusWithResponse(ctx, tx.TxID())
				require.NoError(t, err)

				if statusResponse != nil && statusResponse.JSON200 != nil && statusResponse.JSON200.TxStatus != nil && Status_MINED == *statusResponse.JSON200.TxStatus {
					break checkMinedLoop
				}
			case <-time.NewTimer(15 * time.Second).C:
				t.Fatal("transaction not mined after 15s")
			}
		}
	})
}

func TestPostTx_Success(t *testing.T) {
	tx := createTxHexStringExtended(t) // This is a placeholder for the method to create a valid transaction string.
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, hex.EncodeToString(tx.ExtendedBytes()))
	resp, err := postTx(t, jsonPayload, nil) // no extra headers
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPostTx_RawFormat_Success(t *testing.T) {
	// when
	address, privateKey := fundNewWallet(t)

	utxos := getUtxos(t, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")
	tx, err := createTx(privateKey, address, utxos[0])
	require.NoError(t, err)

	rawFormatHex := hex.EncodeToString(tx.Bytes())

	// then
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, rawFormatHex)
	resp, err := postTx(t, jsonPayload, nil) // no extra headers

	// assert
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPostTx_BadRequest(t *testing.T) {
	jsonPayload := `{"rawTx": "invalidHexData"}` // intentionally malformed
	resp, err := postTx(t, jsonPayload, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request but got: %d", resp.StatusCode)
}

func TestPostTx_MalformedTransaction(t *testing.T) {
	data, err := os.ReadFile("./fixtures/malformedTxHexString.txt")
	require.NoError(t, err)

	txHexString := string(data)
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, txHexString)
	resp, err := postTx(t, jsonPayload, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request but got: %d", resp.StatusCode)
}

func TestPostTx_BadRequestBodyFormat(t *testing.T) {
	improperPayload := `{"transaction": "fakeData"}`

	resp, err := postTx(t, improperPayload, nil) // Using the helper function for the single tx endpoint
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request but got: %d", resp.StatusCode)
}

func postTx(t *testing.T, jsonPayload string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("POST", arcEndpointV1Tx, strings.NewReader(jsonPayload))
	if err != nil {
		t.Fatalf("Error creating HTTP request: %s", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	return client.Do(req)
}

func createTxHexStringExtended(t *testing.T) *bt.Tx {
	address, privateKey := fundNewWallet(t)

	utxos := getUtxos(t, address)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	tx, err := createTx(privateKey, address, utxos[0])
	require.NoError(t, err)

	return tx
}

func TestSubmitMinedTx(t *testing.T) {
	// submit an unregistered, already mined transaction. ARC should return the status as MINED for the transaction.

	// given
	address, _ := fundNewWallet(t)
	utxos := getUtxos(t, address)

	rawTx, _ := bitcoind.GetRawTransaction(utxos[0].Txid)
	tx, _ := bt.NewTxFromString(rawTx.Hex)

	callbackReceivedChan := make(chan *api.TransactionStatus)
	callbackErrChan := make(chan error)

	callbackUrl, token, shutdown := startCallbackSrv(t, callbackReceivedChan, callbackErrChan, nil)
	defer shutdown()

	// when
	_ = postRequest[TransactionResponse](t,
		arcEndpointV1Tx,
		createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}),
		map[string]string{
			"X-WaitFor":       Status_MINED,
			"X-CallbackUrl":   callbackUrl,
			"X-CallbackToken": token,
		},
	)

	// then
	//require.Equal(t, Status_MINED, resp.TxStatus)

	// wait for callback
	callbackTimeout := time.After(10 * time.Second)

	select {
	case status := <-callbackReceivedChan:
		require.Equal(t, rawTx.TxID, status.Txid)
		require.Equal(t, Status_MINED, *status.TxStatus)
	case err := <-callbackErrChan:
		t.Fatalf("callback error: %v", err)
	case <-callbackTimeout:
		t.Fatal("callback exceeded timeout")
	}
}
