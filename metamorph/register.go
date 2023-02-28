package metamorph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TAAL-GmbH/arc/callbacker"
	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/libsv/go-bt/v2"
)

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
