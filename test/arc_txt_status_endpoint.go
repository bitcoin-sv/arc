package test

// import (
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"os"

// 	"testing"
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

// func getTxStatus(t *testing.T, txid string) (*http.Response, error) {
// 	url := fmt.Sprintf("http://arc:9090/v1/tx/%s", txid)
// 	client := &http.Client{}
// 	return client.Get(url)
// }

// func TestGetTxStatus_Mined(t *testing.T) {
// 	txid := "yourSampleTxID" // You can replace this with a specific txid
// 	resp, err := getTxStatus(t, txid)
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		t.Errorf("Expected 200 OK but got: %d", resp.StatusCode)
// 	}

// 	var response struct {
// 		TxStatus string `json:"txStatus"`
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
// 		t.Fatalf("Error decoding response: %v", err)
// 	}

// 	if response.TxStatus != "MINED" {
// 		t.Errorf("Expected txStatus to be 'MINED' but got: %s", response.TxStatus)
// 	}
// }

// //placeholder

// func TestGetTxStatus_Rejected(t *testing.T) {
// 	txid := "yourSampleTxID" // You can replace this with a specific txid
// 	resp, err := getTxStatus(t, txid)
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		t.Errorf("Expected 200 OK but got: %d", resp.StatusCode)
// 	}

// 	var response struct {
// 		TxStatus string `json:"txStatus"`
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
// 		t.Fatalf("Error decoding response: %v", err)
// 	}

// 	if response.TxStatus != "MINED" {
// 		t.Errorf("Expected txStatus to be 'MINED' but got: %s", response.TxStatus)
// 	}
// }

// func TestGetTxStatus_Seen_On_Network(t *testing.T) {
// 	txid := "yourSampleTxID" // You can replace this with a specific txid
// 	resp, err := getTxStatus(t, txid)
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		t.Errorf("Expected 200 OK but got: %d", resp.StatusCode)
// 	}

// 	var response struct {
// 		TxStatus string `json:"txStatus"`
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
// 		t.Fatalf("Error decoding response: %v", err)
// 	}

// 	if response.TxStatus != "MINED" {
// 		t.Errorf("Expected txStatus to be 'MINED' but got: %s", response.TxStatus)
// 	}
// }

// func TestGetTxStatus_NotFound(t *testing.T) {
// 	txid := "nonexistentTxID" // You can replace this with a specific non-existent txid
// 	resp, err := getTxStatus(t, txid)
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusNotFound {
// 		t.Errorf("Expected 404 Not Found but got: %d", resp.StatusCode)
// 	}
// }
