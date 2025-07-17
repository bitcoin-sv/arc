package rpc_client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RPCRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int64       `json:"id"`
	JSONRpc string      `json:"jsonrpc"`
}

type RPCResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Err    interface{}     `json:"error"`
}

func sendJSONRPCCall[T any](ctx context.Context, method string, params []interface{}, nodeHost string, nodePort int, nodeUser, nodePassword string) (*T, error) {
	c := http.Client{}

	rpcRequest := RPCRequest{method, params, time.Now().UnixNano(), "1.0"}
	payloadBuffer := &bytes.Buffer{}
	jsonEncoder := json.NewEncoder(payloadBuffer)

	err := jsonEncoder.Encode(rpcRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx,
		"POST",
		fmt.Sprintf("%s://%s:%d", "http", nodeHost, nodePort),
		payloadBuffer,
	)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(nodeUser, nodePassword)
	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResponse RPCResponse

	if resp.StatusCode != http.StatusOK {
		_ = json.Unmarshal(data, &rpcResponse)
		var msgOk bool
		var msg string
		v, ok := rpcResponse.Err.(map[string]interface{})
		if ok {
			msg, msgOk = v["message"].(string)
		}
		if ok && msgOk {
			return nil, errors.New(msg)
		}
		return nil, errors.New("HTTP error: " + resp.Status)
	}

	err = json.Unmarshal(data, &rpcResponse)
	if err != nil {
		return nil, err
	}

	if rpcResponse.Err != nil {
		e, ok := rpcResponse.Err.(error)
		if ok {
			return nil, e
		}
		return nil, errors.New("unknown error returned from node in rpc response")
	}

	var responseResult T

	err = json.Unmarshal(rpcResponse.Result, &responseResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarhsal response: %v", err)
	}

	return &responseResult, nil
}

type RPCClient struct {
	host     string
	port     int
	user     string
	password string
}

func NewRPCClient(host string, port int, user, password string) (*RPCClient, error) {
	c := &RPCClient{
		host:     host,
		port:     port,
		user:     user,
		password: password,
	}

	return c, nil
}

func (c *RPCClient) GetRawTransactionHex(ctx context.Context, txID string) (string, error) {
	res, err := sendJSONRPCCall[string](ctx, "getrawtransaction", []interface{}{txID, 0}, c.host, c.port, c.user, c.password)
	if err != nil {
		return "", err
	}

	return *res, nil
}

func (c *RPCClient) GetMempoolAncestors(ctx context.Context, txID string) ([]string, error) {
	res, err := sendJSONRPCCall[[]string](ctx, "getmempoolancestors", []interface{}{txID}, c.host, c.port, c.user, c.password)
	if err != nil {
		return nil, err
	}

	return *res, nil
}

func (c *RPCClient) InvalidateBlock(ctx context.Context, blockHash string) error {
	_, err := sendJSONRPCCall[[]byte](ctx, "invalidateblock", []interface{}{blockHash}, c.host, c.port, c.user, c.password)

	return err
}

func (c *RPCClient) SendRawTransaction(ctx context.Context, txHex string) (string, error) {
	res, err := sendJSONRPCCall[string](ctx, "sendrawtransaction", []interface{}{txHex}, c.host, c.port, c.user, c.password)
	if err != nil {
		return "", err
	}
	return *res, nil
}

type VerboseRawTransaction struct {
	TxID          string `json:"txid"`
	BlockHash     string `json:"blockhash,omitempty"`
	Confirmations int    `json:"confirmations,omitempty"`
	// You can expand this if needed (e.g. confirmations, time, etc.)
}

func (c *RPCClient) GetRawTransactionVerbose(ctx context.Context, txID string) (*VerboseRawTransaction, error) {
	res, err := sendJSONRPCCall[VerboseRawTransaction](ctx, "getrawtransaction", []interface{}{txID, true}, c.host, c.port, c.user, c.password)
	if err != nil {
		return nil, err
	}
	return res, nil
}
