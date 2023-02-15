package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// rpcRequest represent a RCP request
type rpcRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int64       `json:"id"`
	JSONRpc string      `json:"jsonrpc"`
}

// rpcResponse represents a RCP response
type rpcResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Err    interface{}     `json:"error"`
}

// DoRPCRequest performs an RPC request to the given Bitcoin node
// this comes from go-bitcoin, which is missing the getsettings method
func DoRPCRequest(serverURL *url.URL, method string, params interface{}) (*NodePolicy, error) {
	rpcR := rpcRequest{method, params, time.Now().UnixNano(), "1.0"}
	payloadBuffer := &bytes.Buffer{}
	jsonEncoder := json.NewEncoder(payloadBuffer)

	err := jsonEncoder.Encode(rpcR)
	if err != nil {
		return nil, fmt.Errorf("failed to encode rpc request: %w", err)
	}

	useUrl := fmt.Sprintf("%s://%s", serverURL.Scheme, serverURL.Host)
	req, err := http.NewRequest("POST", useUrl, payloadBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to create new http request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("Accept", "application/json")

	// Auth ?
	username := serverURL.User.Username()
	password, _ := serverURL.User.Password()
	if len(username) > 0 || len(password) > 0 {
		req.SetBasicAuth(username, password)
	}

	httpClient := http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	response := rpcResponse{}
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	policy := &NodePolicy{}
	err = json.Unmarshal(response.Result, policy)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal policy: %w", err)
	}

	return policy, nil
}
