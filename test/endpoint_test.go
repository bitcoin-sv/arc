package test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoinsv/bsvd/bsvec"
	"github.com/bitcoinsv/bsvutil"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
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
	MerklePath  string      `json:"merklePath"`
}

func TestBatchChainedTxs(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "submit batch of chained transactions - ext format",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			address, privateKey := fundNewWallet(t)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			txs, err := createTxChain(privateKey, utxos[0], 30)
			require.NoError(t, err)

			arcBody := make([]api.TransactionRequest, len(txs))
			for i, tx := range txs {
				arcBody[i] = api.TransactionRequest{
					RawTx: hex.EncodeToString(tx.ExtendedBytes()),
				}
			}

			payLoad, err := json.Marshal(arcBody)
			require.NoError(t, err)

			buffer := bytes.NewBuffer(payLoad)

			// Send POST request
			req, err := http.NewRequest("POST", arcEndpointV1Txs, buffer)
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}

			t.Logf("submitting batch of %d chained txs", len(txs))
			postBatchRequest(t, client, req)

			time.Sleep(1 * time.Second) // give ARC time to perform the status update on DB

			// repeat request to ensure response remains the same
			t.Logf("re-submitting batch of %d chained txs", len(txs))
			postBatchRequest(t, client, req)
		})
	}
}

func postBatchRequest(t *testing.T, client *http.Client, req *http.Request) {
	httpResp, err := client.Do(req)
	require.NoError(t, err)
	defer httpResp.Body.Close()

	require.Equal(t, http.StatusOK, httpResp.StatusCode)

	b, err := io.ReadAll(httpResp.Body)
	require.NoError(t, err)

	bodyResponse := make([]Response, 0)
	err = json.Unmarshal(b, &bodyResponse)
	require.NoError(t, err)

	for i, txResponse := range bodyResponse {
		require.NoError(t, err)
		require.Equalf(t, Status_SEEN_ON_NETWORK, txResponse.TxStatus, "status of tx %d in chain not as expected", i)
	}
}

func createTxChain(privateKey string, utxo0 NodeUnspentUtxo, length int) ([]*bt.Tx, error) {
	batch := make([]*bt.Tx, length)

	utxoTxID := utxo0.Txid
	utxoVout := uint32(utxo0.Vout)
	utxoSatoshis := uint64(utxo0.Amount * 1e8)
	utxoScript := utxo0.ScriptPubKey
	utxoAddress := utxo0.Address

	for i := 0; i < length; i++ {
		tx := bt.NewTx()

		err := tx.From(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
		if err != nil {
			return nil, fmt.Errorf("failed adding input: %v", err)
		}

		amountToSend := utxoSatoshis - feeSat

		recipientScript, err := bscript.NewP2PKHFromAddress(utxoAddress)
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

		batch[i] = tx

		utxoTxID = tx.TxID()
		utxoVout = 0
		utxoSatoshis = amountToSend
		utxoScript = utxo0.ScriptPubKey
	}

	return batch, nil
}

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

func TestPostSkipFee(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "post transaction with skip fee",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			address, privateKey := fundNewWallet(t)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			customFee := uint64(0)

			tx, err := createTx(privateKey, address, utxos[0], customFee)
			require.NoError(t, err)

			fmt.Println("Transaction with Zero fee:", tx)

			arcClient, err := api.NewClientWithResponses(arcEndpoint)
			require.NoError(t, err)

			options := validationOpts{
				skipFeeValidation: true,
			}

			postTxWithHeadersChecksStatus(t, arcClient, tx, Status_SEEN_ON_NETWORK, options)
		})
	}
}

func TestPostSkipTxValidation(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "post transaction with skip tx validation",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			address, privateKey := fundNewWallet(t)
			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			customFee := uint64(0)

			tx, err := createTx(privateKey, address, utxos[0], customFee)
			require.NoError(t, err)

			fmt.Println("Transaction with Zero fee:", tx)

			arcClient, err := api.NewClientWithResponses(arcEndpoint)
			require.NoError(t, err)

			options := validationOpts{
				skipTxValidation: true,
			}

			postTxWithHeadersChecksStatus(t, arcClient, tx, Status_SEEN_ON_NETWORK, options)
		})
	}
}

func Test_E2E_Success(t *testing.T) {
	tx := createTxHexStringExtended(t)
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, hex.EncodeToString(tx.ExtendedBytes()))

	// Send POST request
	req, err := http.NewRequest("POST", arcEndpointV1Tx, strings.NewReader(jsonPayload))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	txID := postSingleRequest(t, client, req)

	time.Sleep(1 * time.Second) // give ARC time to perform the status update on DB

	// repeat request to ensure response remains the same
	txIDRepeat := postSingleRequest(t, client, req)
	require.Equal(t, txID, txIDRepeat)

	// Check transaction status
	statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, txID)
	statusResp, err := http.Get(statusUrl)
	require.NoError(t, err)
	defer statusResp.Body.Close()

	var statusResponse TxStatusResponse
	require.NoError(t, json.NewDecoder(statusResp.Body).Decode(&statusResponse))
	require.Equalf(t, Status_SEEN_ON_NETWORK, statusResponse.TxStatus, "Expected txStatus to be 'SEEN_ON_NETWORK' for tx id %s", txID)

	t.Logf("Transaction status: %s", statusResponse.TxStatus)

	generate(t, 10)

	statusResp, err = http.Get(statusUrl)
	require.NoError(t, err)
	defer statusResp.Body.Close()

	require.NoError(t, json.NewDecoder(statusResp.Body).Decode(&statusResponse))

	require.Equal(t, Status_MINED, statusResponse.TxStatus, "Expected txStatus to be 'MINED'")
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

func postSingleRequest(t *testing.T, client *http.Client, req *http.Request) string {
	httpResp, err := client.Do(req)
	require.NoError(t, err)
	defer httpResp.Body.Close()

	require.Equal(t, http.StatusOK, httpResp.StatusCode)

	var response Response
	require.NoError(t, json.NewDecoder(httpResp.Body).Decode(&response))
	require.Equal(t, Status_SEEN_ON_NETWORK, response.TxStatus)

	return response.Txid
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

	arcClient, _ := api.NewClientWithResponses(arcEndpoint)

	// when
	waitFor := string(Status_MINED)
	params := &api.POSTTransactionParams{
		XWaitFor:       &waitFor,
		XCallbackUrl:   &callbackUrl,
		XCallbackToken: &token,
	}
	arcBody := api.POSTTransactionJSONRequestBody{
		RawTx: hex.EncodeToString(tx.ExtendedBytes()),
	}

	submitResult, submitErr := arcClient.POSTTransactionWithResponse(context.TODO(), params, arcBody)

	// then
	require.NoError(t, submitErr)
	require.Equal(t, http.StatusOK, submitResult.StatusCode())

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

func TestPostCumulativeFeesValidation(t *testing.T) {
	tt := []struct {
		name       string
		options    validationOpts
		lastTxFee  uint64
		shouldPass bool
	}{
		{
			name: "post zero fee txs chain with cumulative fees validation and with skiping fee validation - fee validation is ommited",
			options: validationOpts{
				performCumulativeFeesValidation: true,
				skipFeeValidation:               true,
			},
			shouldPass: true,
		},
		{
			name: "post zero fee tx with cumulative fees validation and with skiping cumulative fee validation - cumulative fee validation is ommited",
			options: validationOpts{
				performCumulativeFeesValidation: false,
			},
			lastTxFee:  10,
			shouldPass: true,
		},
		{
			name: "post  txs chain with too low fee with cumulative fees validation",
			options: validationOpts{
				performCumulativeFeesValidation: true,
			},
			lastTxFee:  1,
			shouldPass: false,
		},
		{
			name: "post  txs chain with suficient fee with cumulative fees validation",
			options: validationOpts{
				performCumulativeFeesValidation: true,
			},
			lastTxFee:  40,
			shouldPass: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// when
			arcClient, err := api.NewClientWithResponses(arcEndpoint)
			require.NoError(t, err)

			address, privateKey := fundNewWallet(t)

			// create mined ancestor
			utxos := getUtxos(t, address)
			require.GreaterOrEqual(t, len(utxos), 1, "No UTXOs available for the address")

			minedTx, err := createTx(privateKey, address, utxos[0], 10)
			require.NoError(t, err)

			// create unmined zero fee ancestors chain
			const zeroFee = uint64(0)
			const zeroChainCount = 3
			ancestors := make([]*bt.Tx, zeroChainCount)

			parentTx := minedTx
			for i := 0; i < zeroChainCount; i++ {
				output := parentTx.Outputs[0]
				utxo := NodeUnspentUtxo{
					Txid:         parentTx.TxID(),
					Vout:         0,
					Address:      address,
					ScriptPubKey: output.LockingScript.String(),
					Amount:       float64(float64(output.Satoshis) / 1e8),
				}

				tx, err := createTx(privateKey, address, utxo, zeroFee)
				require.NoError(t, err)

				ancestors[i] = tx
				parentTx = tx
			}

			// post ancestor transactions
			postTxWithHeadersChecksStatus(t, arcClient, minedTx, Status_SEEN_ON_NETWORK, validationOpts{})
			generate(t, 1) // mine posted transaction

			noValidation := validationOpts{skipTxValidation: true}
			for _, tx := range ancestors {
				postTxWithHeadersChecksStatus(t, arcClient, tx, Status_SEEN_ON_NETWORK, noValidation)
			}

			// then
			// create last transaction in chain
			parentTx = ancestors[len(ancestors)-1]
			output := parentTx.Outputs[0]
			utxo := NodeUnspentUtxo{
				Txid:         parentTx.TxID(),
				Vout:         0,
				Address:      address,
				ScriptPubKey: output.LockingScript.String(),
				Amount:       float64(float64(output.Satoshis) / 1e8),
			}

			childTx, err := createTx(privateKey, address, utxo, tc.lastTxFee)
			require.NoError(t, err)

			response := postTxWithHeaders(t, arcClient, childTx, Status_SEEN_ON_NETWORK, tc.options)

			// assert
			if tc.shouldPass {
				require.Equal(t, int(api.StatusOK), response.StatusCode())
				require.NotNil(t, response.JSON200)
				require.Equal(t, Status_SEEN_ON_NETWORK, response.JSON200.TxStatus)
			} else {
				require.Equal(t, int(api.ErrStatusCumulativeFees), response.StatusCode())
			}
		})
	}
}

type validationOpts struct {
	skipFeeValidation, skipTxValidation, performCumulativeFeesValidation bool
}

func postTxWithHeadersChecksStatus(t *testing.T, client *api.ClientWithResponses, tx *bt.Tx, expectedStatus string, vOpts validationOpts) {
	t.Helper()

	response := postTxWithHeaders(t, client, tx, expectedStatus, vOpts)
	require.Equal(t, http.StatusOK, response.StatusCode())
	require.NotNil(t, response.JSON200)
	require.Equalf(t, expectedStatus, response.JSON200.TxStatus, "status of response: %s does not match expected status: %s for tx ID %s", response.JSON200.TxStatus, expectedStatus, tx.TxID())
}

func postTxWithHeaders(t *testing.T, client *api.ClientWithResponses, tx *bt.Tx, expectedStatus string, vOpts validationOpts) *api.POSTTransactionResponse {
	t.Helper()

	var skipFeeValidationPtr *bool
	if vOpts.skipFeeValidation {
		skipFeeValidationPtr = PtrTo(true)
	}

	var skipTxValidationPtr *bool
	if vOpts.skipTxValidation {
		skipTxValidationPtr = PtrTo(true)
	}

	var performCumulativeFeesValidationPtr *bool
	if vOpts.performCumulativeFeesValidation {
		performCumulativeFeesValidationPtr = PtrTo(true)
	}

	params := &api.POSTTransactionParams{
		XWaitFor:                 &expectedStatus,
		XSkipFeeValidation:       skipFeeValidationPtr,
		XSkipTxValidation:        skipTxValidationPtr,
		XCumulativeFeeValidation: performCumulativeFeesValidationPtr,
	}

	arcBody := api.POSTTransactionJSONRequestBody{
		RawTx: hex.EncodeToString(tx.ExtendedBytes()),
	}

	var response *api.POSTTransactionResponse
	response, err := client.POSTTransactionWithResponse(context.Background(), params, arcBody)
	require.NoError(t, err)

	return response
}
