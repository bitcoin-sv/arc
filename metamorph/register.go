package metamorph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/callbacker"
	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"
)

type RegisterTransactionCallerClient struct {
	blockTxClient blocktx.ClientI
}

func NewRegisterTransactionCallerClient(blockTxClient blocktx.ClientI) *RegisterTransactionCallerClient {
	return &RegisterTransactionCallerClient{
		blockTxClient: blockTxClient,
	}
}

func (r *RegisterTransactionCallerClient) Caller(data *blocktx_api.TransactionAndSource) error {
	if err := r.blockTxClient.RegisterTransaction(context.Background(), data); err != nil {
		return fmt.Errorf("error registering transaction %x: %v", bt.ReverseBytes(data.Hash), err)
	}
	return nil
}

func (r *RegisterTransactionCallerClient) MarshalString(data *blocktx_api.TransactionAndSource) (string, error) {
	return fmt.Sprintf("%x,%s", bt.ReverseBytes(data.Hash), data.Source), nil
}

func (r *RegisterTransactionCallerClient) UnmarshalString(data string) (*blocktx_api.TransactionAndSource, error) {
	parts := strings.Split(data, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("could not unmarshal data: %s", data)
	}

	hash, err := utils.DecodeAndReverseHexString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("could not decode hash: %v", err)
	}

	return &blocktx_api.TransactionAndSource{
		Hash:   hash,
		Source: parts[1],
	}, nil
}

type RegisterCallbackClient struct {
	callbacker callbacker.ClientI
}

func NewRegisterCallbackClient(callbacker callbacker.ClientI) *RegisterCallbackClient {
	return &RegisterCallbackClient{
		callbacker: callbacker,
	}
}

func (r *RegisterCallbackClient) Caller(data *callbacker_api.Callback) error {
	if err := r.callbacker.RegisterCallback(context.Background(), data); err != nil {
		return fmt.Errorf("error registering callback %x: %v", bt.ReverseBytes(data.Hash), err)
	}
	return nil
}

func (r *RegisterCallbackClient) MarshalString(data *callbacker_api.Callback) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *RegisterCallbackClient) UnmarshalString(data string) (*callbacker_api.Callback, error) {
	var callback callbacker_api.Callback
	if err := json.Unmarshal([]byte(data), &callback); err != nil {
		return nil, err
	}
	return &callback, nil
}
