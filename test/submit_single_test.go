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
		address, privateKey := fundNewWallet(t)

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

	tt := []struct {
		name                       string
		numberOfCallbackServers    int
		attemptMultipleSubmissions bool
	}{
		{
			name:                    "post transaction with one callback",
			numberOfCallbackServers: 1,
		},
		{
			name:                    "post transaction with multiple callbacks",
			numberOfCallbackServers: 2,
		},
		{
			name:                       "post transaction with one callback - multiple submissions",
			numberOfCallbackServers:    1,
			attemptMultipleSubmissions: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			address, privateKey := fundNewWallet(t)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			tx, err := createTx(privateKey, address, utxos[0])
			require.NoError(t, err)

			const callbackNumbers = 2 // cannot be greater than 5
			callbackReceivedChannels := make([]chan *TransactionResponse, tc.numberOfCallbackServers)
			callbackErrChannels := make([]chan error, tc.numberOfCallbackServers)
			callbackShutdowns := make([]func(), tc.numberOfCallbackServers)

			for i := 0; i < tc.numberOfCallbackServers; i++ {
				callbackReceivedChan, callbackErrChan, calbackResponseFn := prepareCallback(t, callbackNumbers)
				callbackUrl, token, shutdown := startCallbackSrv(t, callbackReceivedChan, callbackErrChan, calbackResponseFn)
				callbackReceivedChannels[i] = callbackReceivedChan
				callbackErrChannels[i] = callbackErrChan
				callbackShutdowns[i] = shutdown

				testTxSubmission(t, callbackUrl, token, tx)
				// This is to test the multiple submissions with the same callback URL and token
				// Expected behavior is that the callback should not be added to tx and the server should receive the callback only once
				if tc.attemptMultipleSubmissions {
					testTxSubmission(t, callbackUrl, token, tx)
				}
			}

			defer func() {
				for _, shutdown := range callbackShutdowns {
					shutdown()
				}
			}()

			generate(t, 10)

			statusUrl := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx.TxID())
			statusResp := getRequest[TransactionResponse](t, statusUrl)

			require.NotNil(t, statusResp.MerklePath)
			_, err = bc.NewBUMPFromStr(*statusResp.MerklePath)
			require.NoError(t, err)

			for i := 0; i < tc.numberOfCallbackServers; i++ {
				t.Logf("callback %d", i)
				for j := 0; j < callbackNumbers; j++ {
					t.Logf("callback iteration %d", j)
					callbackTimeout := time.After(time.Second * time.Duration(j+1) * 2 * 5)

					select {
					case callback := <-callbackReceivedChannels[i]:
						require.NotNil(t, callback)

						t.Logf("Callback %d iteration %d result: %s", i, j, callback.TxStatus)
						require.Equal(t, statusResp.Txid, callback.Txid)
						require.Equal(t, *statusResp.BlockHeight, *callback.BlockHeight)
						require.Equal(t, *statusResp.BlockHash, *callback.BlockHash)
						require.Equal(t, Status_MINED, callback.TxStatus)

					case err := <-callbackErrChannels[i]:
						t.Fatalf("callback %d received - failed to parse callback %v", i, err)
					case <-callbackTimeout:
						t.Fatalf("callback %d not received - timeout", i)
					}
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

		expectedStatusCode int
	}{
		{
			name:              "post transaction with without fee validation",
			skipFeeValidation: true,
			skipTxValidation:  false,

			expectedStatusCode: http.StatusOK,
		},
		{
			name:              "post low fee tx without tx validation",
			skipFeeValidation: false,
			skipTxValidation:  true,

			expectedStatusCode: http.StatusOK,
		},
		{
			name:              "post low fee tx with fee validation",
			skipFeeValidation: false,
			skipTxValidation:  false,

			expectedStatusCode: 465,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			address, privateKey := fundNewWallet(t)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			fee := uint64(0)

			lowFeeTx, err := createTx(privateKey, address, utxos[0], fee)
			require.NoError(t, err)

			resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(lowFeeTx.ExtendedBytes())}),
				map[string]string{
					"X-WaitFor":           Status_SEEN_ON_NETWORK,
					"X-SkipFeeValidation": strconv.FormatBool(tc.skipFeeValidation),
					"X-SkipTxValidation":  strconv.FormatBool(tc.skipTxValidation),
				}, tc.expectedStatusCode)

			if tc.expectedStatusCode == http.StatusOK {
				require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)
			}
		})
	}
}

func TestPostCumulativeFeesValidation(t *testing.T) {
	type validationOpts struct {
		performCumulativeFeesValidation bool
		skipFeeValidation               bool
	}

	tt := []struct {
		name      string
		options   validationOpts
		lastTxFee uint64
		chainLong int

		expectedStatusCode int
		expectedErrInfo    string
	}{
		{
			name: "post zero fee txs chain with cumulative fees validation and with skiping fee validation - fee validation is ommited",
			options: validationOpts{
				performCumulativeFeesValidation: true,
				skipFeeValidation:               true,
			},
			expectedStatusCode: 200,
		},
		{
			name: "post zero fee tx with cumulative fees validation and with skiping cumulative fee validation - cumulative fee validation is ommited",
			options: validationOpts{
				performCumulativeFeesValidation: false,
			},
			lastTxFee:          17,
			expectedStatusCode: 200,
		},
		{
			name: "post  txs chain with too low fee with cumulative fees validation",
			options: validationOpts{
				performCumulativeFeesValidation: true,
			},
			lastTxFee:          1,
			expectedStatusCode: 473,
			expectedErrInfo:    "arc error 473: cumulative transaction fee of 1 sat is too low",
		},
		{
			name: "post  txs chain with suficient fee with cumulative fees validation",
			options: validationOpts{
				performCumulativeFeesValidation: true,
			},
			lastTxFee:          90,
			expectedStatusCode: 200,
		},
		{
			name: "post  txs chain with cumulative fees validation - chain too long",
			options: validationOpts{
				performCumulativeFeesValidation: true,
			},
			lastTxFee:          247,
			chainLong:          25,
			expectedStatusCode: 473,
			expectedErrInfo:    "arc error 473: too many unconfirmed parents, 25 [limit: 25]",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// when
			address, privateKey := getNewWalletAddress(t)

			// create mined ancestors
			const minedAncestorsCount = 2
			var minedAncestors []*bt.Tx

			sendToAddress(t, address, 0.0001)
			sendToAddress(t, address, 0.00011)
			utxos := getUtxos(t, address)
			require.GreaterOrEqual(t, len(utxos), minedAncestorsCount, "No UTXOs available for the address")

			for i := range minedAncestorsCount {
				minedTx, err := createTx(privateKey, address, utxos[i], 10)
				require.NoError(t, err)

				minedAncestors = append(minedAncestors, minedTx)
			}

			// create unmined zero fee ancestors graph
			const zeroFee = uint64(0)
			const defaultZeroChainCount = 3
			zeroChainCount := defaultZeroChainCount
			if tc.chainLong > 0 {
				zeroChainCount = tc.chainLong / 2
			}

			var zeroFeeChains [][]*bt.Tx
			for i, minedTx := range minedAncestors {
				if i+1 == len(minedAncestors) {
					zeroChainCount++
				}

				chain := make([]*bt.Tx, zeroChainCount)
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

					chain[i] = tx
					parentTx = tx
				}

				zeroFeeChains = append(zeroFeeChains, chain)
			}

			// post ancestor transactions
			for _, tx := range minedAncestors {
				body := TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}
				resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, body),
					map[string]string{"X-WaitFor": Status_SEEN_ON_NETWORK}, 200)

				require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)
			}
			generate(t, 1) // mine posted transactions

			for _, chain := range zeroFeeChains {
				for _, tx := range chain {
					body := TransactionRequest{RawTx: hex.EncodeToString(tx.ExtendedBytes())}
					resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, body),
						map[string]string{
							"X-WaitFor":           Status_SEEN_ON_NETWORK,
							"X-SkipFeeValidation": strconv.FormatBool(true),
						}, 200)

					require.Equal(t, Status_SEEN_ON_NETWORK, resp.TxStatus)
				}
			}

			// then
			// create last transaction
			var nodeUtxos []NodeUnspentUtxo
			for _, chain := range zeroFeeChains {
				// get otput from the lastes tx in the chain
				parentTx := chain[len(chain)-1]
				output := parentTx.Outputs[0]
				utxo := NodeUnspentUtxo{
					Txid:         parentTx.TxID(),
					Vout:         0,
					Address:      address,
					ScriptPubKey: output.LockingScript.String(),
					Amount:       float64(float64(output.Satoshis) / 1e8),
				}

				nodeUtxos = append(nodeUtxos, utxo)
			}

			lastTx, err := createTxFrom(privateKey, address, nodeUtxos, tc.lastTxFee)
			require.NoError(t, err)

			response := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: hex.EncodeToString(lastTx.ExtendedBytes())}),
				map[string]string{
					"X-WaitFor":                 Status_SEEN_ON_NETWORK,
					"X-CumulativeFeeValidation": strconv.FormatBool(tc.options.performCumulativeFeesValidation),
					"X-SkipFeeValidation":       strconv.FormatBool(tc.options.skipFeeValidation),
				}, tc.expectedStatusCode)

			// assert
			if tc.expectedStatusCode == http.StatusOK {
				require.Equal(t, Status_SEEN_ON_NETWORK, response.TxStatus)
			} else {
				require.Contains(t, *response.ExtraInfo, tc.expectedErrInfo)
			}
		})
	}
}
