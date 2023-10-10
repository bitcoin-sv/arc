package test

// import (
// 	"encoding/json"
// 	"net/http"

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

// //this tests does a full end to end checking the status at each point

// func getPolicySettings(t *testing.T) (*http.Response, error) {
// 	url := "http://arc:9090/v1/policy"
// 	client := &http.Client{}
// 	return client.Get(url)
// }

// func TestGetPolicySettings_Success(t *testing.T) {
// 	resp, err := getPolicySettings(t)
// 	if err != nil {
// 		t.Fatalf("Error sending HTTP request: %s", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		t.Errorf("Expected 200 OK but got: %d", resp.StatusCode)
// 	}

// 	var response struct {
// 		Policy struct {
// 			MaxScriptSizePolicy    int64  `json:"maxscriptsizepolicy"`
// 			MaxTxSigOpsCountPolicy uint64 `json:"maxtxsigopscountspolicy"`
// 			MaxTxSizePolicy        int64  `json:"maxtxsizepolicy"`
// 			MiningFee              struct {
// 				Satoshis int64 `json:"satoshis"`
// 				Bytes    int64 `json:"bytes"`
// 			} `json:"miningFee"`
// 		} `json:"policy"`
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
// 		t.Fatalf("Error decoding response: %v", err)
// 	}

// 	// Optional: You can further verify specific fields within the returned policy, if desired.
// }
