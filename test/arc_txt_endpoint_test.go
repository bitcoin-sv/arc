package test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/handler"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoinsv/bsvd/bsvec"
	"github.com/bitcoinsv/bsvutil"
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
}

func TestMain(m *testing.M) {
	info, err := bitcoind.GetInfo()
	if err != nil {
		log.Fatalf("failed to get info: %v", err)
	}

	log.Printf("current block height: %d", info.Blocks)

	os.Exit(m.Run())
}

func createTx(privateKey string, address string, utxo NodeUnspentUtxo) (*bt.Tx, error) {
	tx := bt.NewTx()

	// Add an input using the first UTXO
	utxoTxID := utxo.Txid
	utxoVout := uint32(utxo.Vout)
	utxoSatoshis := uint64(utxo.Amount * 1e8) // Convert BTC to satoshis
	utxoScript := utxo.ScriptPubKey

	err := tx.From(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
	if err != nil {
		return nil, fmt.Errorf("failed adding input: %v", err)
	}

	// Add an output to the address you've previously created
	recipientAddress := address
	amountToSend := uint64(1) // Example value - 0.009 BTC (taking fees into account)

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
			address, privateKey := getNewWalletAddress(t)

			generate(t, 100)

			t.Logf("generated address: %s", address)

			sendToAddress(t, address, 0.001)

			txID := sendToAddress(t, address, 0.02)
			t.Logf("sent 0.02 BSV to: %s", txID)

			hash := generate(t, 1)
			t.Logf("generated 1 block: %s", hash)

			utxos := getUtxos(t, address)
			require.True(t, len(utxos) > 0, "No UTXOs available for the address")

			tx, err := createTx(privateKey, address, utxos[0])
			require.NoError(t, err)

			url := "http://arc:9090/"

			arcClient, err := api.NewClientWithResponses(url)
			require.NoError(t, err)

			ctx := context.Background()

			hostname, err := os.Hostname()
			require.NoError(t, err)

			waitForStatus := api.WaitForStatus(metamorph_api.Status_SEEN_ON_NETWORK)
			params := &api.POSTTransactionParams{
				XWaitForStatus: &waitForStatus,
				XCallbackUrl:   handler.PtrTo(fmt.Sprintf("http://%s:9000/callback", hostname)),
				XCallbackToken: handler.PtrTo("1234"),
			}

			arcBody := api.POSTTransactionJSONRequestBody{
				RawTx: hex.EncodeToString(tx.ExtendedBytes()),
			}

			var response *api.POSTTransactionResponse
			response, err = arcClient.POSTTransactionWithResponse(ctx, params, arcBody)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, response.StatusCode())
			require.NotNil(t, response.JSON200)
			require.Equal(t, "SEEN_ON_NETWORK", response.JSON200.TxStatus)

			callbackReceivedChan := make(chan *api.TransactionStatus, 2)
			errChan := make(chan error, 2)

			expectedAuthHeader := "Bearer 1234"
			srv := &http.Server{Addr: ":9000"}
			defer func() {
				t.Log("shutting down callback listener")
				if err = srv.Shutdown(context.TODO()); err != nil {
					panic(err)
				}
			}()

			iterations := 0
			http.HandleFunc("/callback", func(w http.ResponseWriter, req *http.Request) {

				defer func() {
					err := req.Body.Close()
					if err != nil {
						t.Log("failed to close body")
					}
				}()

				bodyBytes, err := io.ReadAll(req.Body)
				if err != nil {
					errChan <- err
				}

				var status api.TransactionStatus
				err = json.Unmarshal(bodyBytes, &status)
				if err != nil {
					errChan <- err
				}

				if expectedAuthHeader != req.Header.Get("Authorization") {
					errChan <- fmt.Errorf("auth header %s not as expected %s", expectedAuthHeader, req.Header.Get("Authorization"))
				}

				// Let ARC send the callback 2 times. First one fails.
				if iterations == 0 {
					t.Log("callback received, responding bad request")

					err = respondToCallback(w, false)
					if err != nil {
						t.Fatalf("Failed to respond to callback: %v", err)
					}

					callbackReceivedChan <- &status

					iterations++
					return
				}

				t.Log("callback received, responding success")

				err = respondToCallback(w, true)
				if err != nil {
					t.Fatalf("Failed to respond to callback: %v", err)
				}
				callbackReceivedChan <- &status
			})

			go func(server *http.Server) {
				t.Log("starting callback server")
				err = server.ListenAndServe()
				if err != nil {
					return
				}
			}(srv)

			generate(t, 10)

			var statusResopnse *api.GETTransactionStatusResponse
			statusResopnse, err = arcClient.GETTransactionStatusWithResponse(ctx, response.JSON200.Txid)

			for i := 0; i <= 1; i++ {
				t.Logf("callback iteration %d", i)
				select {
				case callback := <-callbackReceivedChan:
					require.Equal(t, statusResopnse.JSON200.Txid, callback.Txid)
					require.Equal(t, statusResopnse.JSON200.BlockHeight, callback.BlockHeight)
					require.Equal(t, statusResopnse.JSON200.BlockHash, callback.BlockHash)
				case err := <-errChan:
					t.Fatalf("callback received - failed to parse callback %v", err)
				case <-time.NewTicker(time.Second * 15).C:
					t.Fatal("callback not received")
				}
			}
		})
	}
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

	jsonResp, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	_, err = w.Write(jsonResp)
	if err != nil {
		return err
	}
	return nil
}

func TestHttpPost(t *testing.T) {
	address, privateKey := getNewWalletAddress(t)

	generate(t, 100)

	fmt.Println(address)

	sendToAddress(t, address, 0.001)

	txID := sendToAddress(t, address, 0.02)
	hash := generate(t, 1)

	fmt.Println(txID)
	fmt.Println(hash)

	utxos := getUtxos(t, address)
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

	wif, err := bsvutil.DecodeWIF(privateKey)
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
	bodyBytes, err := io.ReadAll(resp.Body)
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

	generate(t, 10)

	statusUrl := fmt.Sprintf("http://arc:9090/v1/tx/%s", response.Txid)
	statusResp, err := http.Get(statusUrl)
	if err != nil {
		t.Fatalf("Error sending GET request to /v1/tx/{txid}: %s", err)
	}
	defer statusResp.Body.Close()

	statusBodyBytes, err := io.ReadAll(statusResp.Body)
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

	statusBodyBytes, err = io.ReadAll(statusResp.Body) // <-- Use "=" instead of ":="
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

func TestPostTx_Success(t *testing.T) {
	txHexString := createTxHexStringExtended(t) // This is a placeholder for the method to create a valid transaction string.
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, txHexString)
	resp, err := postTx(t, jsonPayload, nil) // no extra headers
	if err != nil {
		t.Fatalf("Error sending HTTP request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK but got: %d", resp.StatusCode)
	}
}

func TestPostTx_BadRequest(t *testing.T) {
	jsonPayload := `{"rawTx": "invalidHexData"}` // intentionally malformed
	resp, err := postTx(t, jsonPayload, nil)
	if err != nil {
		t.Fatalf("Error sending HTTP request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request but got: %d", resp.StatusCode)
	}
}

func TestPostTx_MalformedTransaction(t *testing.T) {
	txHexString := "010000000000000000ef02fa868734c4b8997f83388158ba0c8750aa1ce0405063305d08e41d763727f9aa00000000fd090751145eb2169818a110149787ed09a918c842aad88352023703145eb2169818a110149787ed09a918c842aad883520020880c6b446419591192f4f3146b7002dc40e81cfc7256732ab1d8fa8d1e72510f004d4b06010000000f052f366174eec73ee92067ff31373ca997e3fe48f15383e37fc734a0a24ca0752adad0a7b9ceca853768aebb6965eca126a62965f698a0c1bc43d83db632adfa868734c4b8997f83388158ba0c8750aa1ce0405063305d08e41d763727f9aa00000000fdac0576a9145eb2169818a110149787ed09a918c842aad8835288ac6976aa607f5f7f7c5e7f7c5d7f7c5c7f7c5b7f7c5a7f7c597f7c587f7c577f7c567f7c557f7c547f7c537f7c527f7c517f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7c5f7f7c5e7f7c5d7f7c5c7f7c5b7f7c5a7f7c597f7c587f7c577f7c567f7c557f7c547f7c537f7c527f7c517f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e01007e818b21414136d08c5ed2bf3ba048afe6dcaebafeffffffffffffffffffffffffffffff007d976e7c5296a06394677768827601249301307c7e23022079be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798027e7c7e7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e01417e21038ff83d8cf12121491609c4939dc11c4aa35503508fe432dc5a5c1905608b9218ad547f7701207f01207f7701247f517f7801007e8102fd00a063546752687f7801007e817f727e7b01177f777b557a766471567a577a786354807e7e676d68aa880067765158a569765187645294567a5379587a7e7e78637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6867567a6876aa587a7d54807e577a597a5a7a786354807e6f7e7eaa727c7e676d6e7eaa7c687b7eaa587a7d877663516752687c72879b69537a647500687c7b547f77517f7853a0916901247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f77788c6301247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f777852946301247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f77686877517f7c52797d8b9f7c53a09b91697c76638c7c587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f777c6876638c7c587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f777c6863587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f7768587f517f7801007e817602fc00a06302fd00a063546752687f7801007e81727e7b7b687f75537f7c0376a9148801147f775379645579887567726881766968789263556753687a76026c057f7701147f8263517f7c766301007e817f7c6775006877686b537992635379528763547a6b547a6b677c6b567a6b537a7c717c71716868547a587f7c81547a557964936755795187637c686b687c547f7701207f75748c7a7669765880748c7a76567a876457790376a9147e7c7e557967041976a9147c7e0288ac687e7e5579636c766976748c7a9d58807e6c0376a9147e748c7a7e6c7e7e676c766b8263828c007c80517e846864745aa0637c748c7a76697d937b7b58807e56790376a9147e748c7a7e55797e7e6868686c567a5187637500678263828c007c80517e846868647459a0637c748c7a76697d937b7b58807e55790376a9147e748c7a7e55797e7e687459a0637c748c7a76697d937b7b58807e55790376a9147e748c7a7e55797e7e68687c537a9d547963557958807e041976a91455797e0288ac7e7e68aa87726d77776a14704c123b485438548ed86b42271f16f59147083f01000a544f4b454e54455354530754455354494e470100000000000000ffffffffb5f093629fbdb1a0c3b4aafbdf3eed605635aceb16b10544c3cfd8e20f4b13250000000041000000473044022001dbc61d9535f130aad7ce7bed4910cbfbe817e01f21e5f365df0bf75fca9e7c022045b3cd555076c591b1d93019e4796e77ad14291f0faecbc1e200089bf493bc22412102b078df114648b7e576eac0ba1e6caa874c29fd0836bbbd31b2e6241b8dd298e4ffffffff0100000000000000fdac0576a9145eb2169818a110149787ed09a918c842aad8835288ac6976aa607f5f7f7c5e7f7c5d7f7c5c7f7c5b7f7c5a7f7c597f7c587f7c577f7c567f7c557f7c547f7c537f7c527f7c517f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7c5f7f7c5e7f7c5d7f7c5c7f7c5b7f7c5a7f7c597f7c587f7c577f7c567f7c557f7c547f7c537f7c527f7c517f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e01007e818b21414136d08c5ed2bf3ba048afe6dcaebafeffffffffffffffffffffffffffffff007d976e7c5296a06394677768827601249301307c7e23022079be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798027e7c7e7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e01417e21038ff83d8cf12121491609c4939dc11c4aa35503508fe432dc5a5c1905608b9218ad547f7701207f01207f7701247f517f7801007e8102fd00a063546752687f7801007e817f727e7b01177f777b557a766471567a577a786354807e7e676d68aa880067765158a569765187645294567a5379587a7e7e78637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6867567a6876aa587a7d54807e577a597a5a7a786354807e6f7e7eaa727c7e676d6e7eaa7c687b7eaa587a7d877663516752687c72879b69537a647500687c7b547f77517f7853a0916901247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f77788c6301247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f777852946301247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f77686877517f7c52797d8b9f7c53a09b91697c76638c7c587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f777c6876638c7c587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f777c6863587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f7768587f517f7801007e817602fc00a06302fd00a063546752687f7801007e81727e7b7b687f75537f7c0376a9148801147f775379645579887567726881766968789263556753687a76026c057f7701147f8263517f7c766301007e817f7c6775006877686b537992635379528763547a6b547a6b677c6b567a6b537a7c717c71716868547a587f7c81547a557964936755795187637c686b687c547f7701207f75748c7a7669765880748c7a76567a876457790376a9147e7c7e557967041976a9147c7e0288ac687e7e5579636c766976748c7a9d58807e6c0376a9147e748c7a7e6c7e7e676c766b8263828c007c80517e846864745aa0637c748c7a76697d937b7b58807e56790376a9147e748c7a7e55797e7e6868686c567a5187637500678263828c007c80517e846868647459a0637c748c7a76697d937b7b58807e55790376a9147e748c7a7e55797e7e687459a0637c748c7a76697d937b7b58807e55790376a9147e748c7a7e55797e7e68687c537a9d547963557958807e041976a91455797e0288ac7e7e68aa87726d77776a14704c123b485438548ed86b42271f16f59147083f01000a544f4b454e54455354530754455354494e47880c6b446419591192f4f3146b7002dc40e81cfc7256732ab1d8fa8d1e72510f000000006a47304402203ddb9632903601ac7364eefbb8cc4604dd4bb0259e82f643c438f24c72d6f1e902207a43724899d6ef4bcf0876da32a09df28274640f60098149cb9a4bfe4ffb462a412102b078df114648b7e576eac0ba1e6caa874c29fd0836bbbd31b2e6241b8dd298e4ffffffffe8030000000000001976a9145eb2169818a110149787ed09a918c842aad8835288ac020100000000000000fdac0576a9145eb2169818a110149787ed09a918c842aad8835288ac6976aa607f5f7f7c5e7f7c5d7f7c5c7f7c5b7f7c5a7f7c597f7c587f7c577f7c567f7c557f7c547f7c537f7c527f7c517f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7c5f7f7c5e7f7c5d7f7c5c7f7c5b7f7c5a7f7c597f7c587f7c577f7c567f7c557f7c547f7c537f7c527f7c517f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e01007e818b21414136d08c5ed2bf3ba048afe6dcaebafeffffffffffffffffffffffffffffff007d976e7c5296a06394677768827fg1249301307c7e23022079be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798027e7c7e7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c8276638c687f7c7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e7e01417e21038ff83d8cf12121491609c4939dc11c4aa35503508fe432dc5a5c1905608b9218ad547f7701207f01207f7701247f517f7801007e8102fd00a063546752687f7801007e817f727e7b01177f777b557a766471567a577a786354807e7e676d68aa880067765158a569765187645294567a5379587a7e7e78637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6878637c8c7c53797e577a7e6867567a6876aa587a7d54807e577a597a5a7a786354807e6f7e7eaa727c7e676d6e7eaa7c687b7eaa587a7d877663516752687c72879b69537a647500687c7b547f77517f7853a0916901247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f77788c6301247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f777852946301247f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e816854937f77686877517f7c52797d8b9f7c53a09b91697c76638c7c587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f777c6876638c7c587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f777c6863587f77517f7c01007e817602fc00a06302fd00a063546752687f7c01007e81687f7768587f517f7801007e817602fc00a06302fd00a063546752687f7801007e81727e7b7b687f75537f7c0376a9148801147f775379645579887567726881766968789263556753687a76026c057f7701147f8263517f7c766301007e817f7c6775006877686b537992635379528763547a6b547a6b677c6b567a6b537a7c717c71716868547a587f7c81547a557964936755795187637c686b687c547f7701207f75748c7a7669765880748c7a76567a876457790376a9147e7c7e557967041976a9147c7e0288ac687e7e5579636c766976748c7a9d58807e6c0376a9147e748c7a7e6c7e7e676c766b8263828c007c80517e846864745aa0637c748c7a76697d937b7b58807e56790376a9147e748c7a7e55797e7e6868686c567a5187637500678263828c007c80517e846868647459a0637c748c7a76697d937b7b58807e55790376a9147e748c7a7e55797e7e687459a0637c748c7a76697d937b7b58807e55790376a9147e748c7a7e55797e7e68687c537a9d547963557958807e041976a91455797e0288ac7e7e68aa87726d77776a14704c123b485438548ed86b42271f16f59147083f01000a544f4b454e54455354530754455354494e4737030000000000001976a9145eb2169818a110149787ed09a918c842aad8835288ac00000000"
	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, txHexString)
	resp, err := postTx(t, jsonPayload, nil)
	if err != nil {
		t.Fatalf("Error sending HTTP request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Errorf("Expected 422 Unprocessable Entity but got: %d", resp.StatusCode)
	}
}

func TestPostTx_BadRequestBodyFormat(t *testing.T) {
	// Instead of sending a valid hex string, send a JSON
	improperPayload := `{"transaction": "fakeData"}`

	resp, err := postTx(t, improperPayload, nil) // Using the helper function for the single tx endpoint
	if err != nil {
		t.Fatalf("Error sending HTTP request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request due to improper body format but got: %d", resp.StatusCode)
	}
}

func TestPostTx_ConflictingTx(t *testing.T) {
	txHexString := createTxHexStringExtended(t) // This is a placeholder for the method to create a valid transaction string.
	conflictingtxt := createTxHexStringExtended(t) // This is a placeholder for the method to create a valid transaction string.

	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, txHexString)
	resp, err := postTx(t, jsonPayload, nil) // no extra headers
	if err != nil {
		t.Fatalf("Error sending HTTP request: %s", err)
	}
	defer resp.Body.Close()

	jsonPayload := fmt.Sprintf(`{"rawTx": "%s"}`, conflictingtxt)
	resp, err := postTx(t, jsonPayload, nil) // no extra headers
	if err != nil {
		t.Fatalf("Error sending HTTP request: %s", err)
	}
	defer resp.Body.Close()


	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK but got: %d", resp.StatusCode)
	}


	if resp.StatusCode != 466 {
		t.Errorf("Expected 466 Conflicting transaction found but got: %d", resp.StatusCode)
	}
}

func postTx(t *testing.T, jsonPayload string, headers map[string]string) (*http.Response, error) {
	url := "http://arc:9090/v1/tx"
	req, err := http.NewRequest("POST", url, strings.NewReader(jsonPayload))
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

func createTxHexStringExtended(t *testing.T) string {
	address, privateKey := getNewWalletAddress(t)

	generate(t, 100)

	fmt.Println(address)

	sendToAddress(t, address, 0.001)

	txID := sendToAddress(t, address, 0.02)
	hash := generate(t, 1)

	fmt.Println(txID)
	fmt.Println(hash)

	utxos := getUtxos(t, address)
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
	return hex.EncodeToString(extBytes)
}
