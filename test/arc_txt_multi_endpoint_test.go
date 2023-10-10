package test

// import (
// 	"context"
// 	"encoding/hex"
// 	"encoding/json"
// 	"fmt"
// 	"io/ioutil"
// 	"log"
// 	"net/http"
// 	"os"
// 	"strings"
// 	"time"

// 	"testing"

// 	"github.com/bitcoinsv/bsvd/bsvec"
// 	"github.com/btcsuite/btcd/btcutil"
// 	"github.com/libsv/go-bk/bec"
// 	"github.com/libsv/go-bt/v2"
// 	"github.com/libsv/go-bt/v2/bscript"
// 	"github.com/libsv/go-bt/v2/unlocker"
// )

// type Response struct {
// 	BlockHash   string `json:"blockHash"`
// 	BlockHeight int    `json:"blockHeight"`
// 	ExtraInfo   string `json:"extraInfo"`
// 	Status      int    `json:"status"`
// 	Timestamp   string `json:"timestamp"`
// 	Title       string `json:"title"`
// 	TxStatus    string `json:"txStatus"`
// 	Txid        string `json:"txid"`
// }

// type TxStatusResponse struct {
// 	BlockHash   string      `json:"blockHash"`
// 	BlockHeight int         `json:"blockHeight"`
// 	ExtraInfo   interface{} `json:"extraInfo"` // It could be null or any type, so we use interface{}
// 	Timestamp   string      `json:"timestamp"`
// 	TxStatus    string      `json:"txStatus"`
// 	Txid        string      `json:"txid"`
// }

// func TestMain(m *testing.M) {
// 	info, err := bitcoind.GetInfo()
// 	if err != nil {
// 		log.Fatalf("failed to get info: %v", err)
// 	}

// 	log.Printf("current block height: %d", info.Blocks)

// 	os.Exit(m.Run())
// }

// //this tests does a full end to end checking the status at each point

// func postTxs(t *testing.T, txHexStrings []string, headers map[string]string) (*http.Response, error) {
// 	url := "http://arc:9090/v1/txs"
// 	jsonPayload := strings.Join(txHexStrings, "\n") // Separate each transaction hex string with a newline

// 	req, err := http.NewRequest("POST", url, strings.NewReader(jsonPayload))
// 	if err != nil {
// 		t.Fatalf("Error creating HTTP request: %s", err)
// 	}

// 	// Set headers
// 	req.Header.Set("Content-Type", "text/plain")
// 	for key, value := range headers {
// 		req.Header.Set(key, value)
// 	}

// 	client := &http.Client{}
// 	return client.Do(req)
// }

// func TestPostTxs_Success(t *testing.T) {
// 	txHexStrings := createValidTxs(t, 2)       // Placeholder method to create an array of two valid transaction strings.
// 	resp, err := postTxs(t, txHexStrings, nil) // no extra headers
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		t.Errorf("Expected 200 OK but got: %d", resp.StatusCode)
// 	}
// }

// func TestPostTxs_BadRequest(t *testing.T) {
// 	txHexStrings := []string{"invalidHexData1", "invalidHexData2"} // Intentionally malformed
// 	resp, err := postTxs(t, txHexStrings, nil)
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusBadRequest {
// 		t.Errorf("Expected 400 Bad Request but got: %d", resp.StatusCode)
// 	}
// }

// func TestPostTxs_SecurityFail(t *testing.T) {
// 	txHexStrings := createValidTxs(t, 2) // Placeholder method to create an array of two valid transaction strings.
// 	headers := map[string]string{
// 		"X-CallbackToken": "invalidToken", // Assume this triggers a security failure
// 	}
// 	resp, err := postTxs(t, txHexStrings, headers)
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusForbidden {
// 		t.Errorf("Expected 403 Forbidden but got: %d", resp.StatusCode)
// 	}
// }

// func TestPostTxs_BadRequestBodyFormat(t *testing.T) {
// 	// Instead of sending a newline-separated list of hex strings, send a JSON
// 	improperPayload := `{"transaction": "fakeData"}`

// 	resp, err := postTxs(t, []string{improperPayload}, nil)
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusBadRequest {
// 		t.Errorf("Expected 400 Bad Request due to improper body format but got: %d", resp.StatusCode)
// 	}
// }
