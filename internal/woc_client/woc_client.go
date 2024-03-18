package woc_client

import (
	"container/list"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

type WocClient struct {
	client        http.Client
	authorization string
}

func WithAuth(authorization string) func(*WocClient) {
	return func(p *WocClient) {
		p.authorization = authorization
	}
}

func New(opts ...func(client *WocClient)) *WocClient {
	w := &WocClient{
		client: http.Client{Timeout: 10 * time.Second},
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

type wocUtxo struct {
	Txid     string `json:"tx_hash"`
	Vout     uint32 `json:"tx_pos"`
	Height   uint32 `json:"height"`
	Satoshis uint64 `json:"value"`
}

type wocBalance struct {
	Confirmed   int64 `json:"confirmed"`
	Unconfirmed int64 `json:"unconfirmed"`
}

// GetUTXOs Get UTXOs from WhatsOnChain
func (w *WocClient) GetUTXOs(mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error) {
	net := "test"
	if mainnet {
		net = "main"
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent", net, address), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to crreate request: %v", err)
	}

	if w.authorization != "" {
		req.Header.Set("Authorization", w.authorization)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to get utxos failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("response status not OK: %s", resp.Status)
	}

	var wocUnspent []*wocUtxo
	err = json.NewDecoder(resp.Body).Decode(&wocUnspent)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	unspent := make([]*bt.UTXO, len(wocUnspent))
	for i, utxo := range wocUnspent {
		txIDBytes, err := hex.DecodeString(utxo.Txid)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex string: %v", err)
		}

		unspent[i] = &bt.UTXO{
			TxID:          txIDBytes,
			Vout:          utxo.Vout,
			LockingScript: lockingScript,
			Satoshis:      utxo.Satoshis,
		}
	}

	return unspent, nil
}

// GetUTXOsList Get UTXOs from WhatsOnChain as a list
func (w *WocClient) GetUTXOsList(mainnet bool, lockingScript *bscript.Script, address string) (*list.List, error) {
	net := "test"
	if mainnet {
		net = "main"
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent", net, address), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to crreate request: %v", err)
	}

	if w.authorization != "" {
		req.Header.Set("Authorization", w.authorization)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to get utxos failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("response status not OK: %s", resp.Status)
	}

	var wocUnspent []*wocUtxo
	err = json.NewDecoder(resp.Body).Decode(&wocUnspent)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	values := list.New()
	for _, utxo := range wocUnspent {
		txIDBytes, err := hex.DecodeString(utxo.Txid)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex string: %v", err)
		}

		btUtxo := &bt.UTXO{
			TxID:          txIDBytes,
			Vout:          utxo.Vout,
			LockingScript: lockingScript,
			Satoshis:      utxo.Satoshis,
		}

		values.PushBack(btUtxo)
	}

	return values, nil
}

func (w *WocClient) GetBalance(mainnet bool, address string) (int64, error) {
	net := "test"
	if mainnet {
		net = "main"
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/balance", net, address), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to crreate request: %v", err)
	}

	if w.authorization != "" {
		req.Header.Set("Authorization", w.authorization)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request to get balance failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("response status not OK: %s", resp.Status)
	}

	var balance wocBalance
	err = json.NewDecoder(resp.Body).Decode(&balance)
	if err != nil {
		return 0, fmt.Errorf("failed to decode response: %v", err)
	}

	return balance.Confirmed + balance.Unconfirmed, nil
}

func (w *WocClient) TopUp(mainnet bool, address string) error {
	net := "test"
	if mainnet {
		return errors.New("top up can only be done on testnet")
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api-test.whatsonchain.com/v1/bsv/%s/faucet/send/%s", net, address), nil)
	if err != nil {
		return fmt.Errorf("failed to crreate request: %v", err)
	}

	if w.authorization != "" {
		req.Header.Set("Authorization", w.authorization)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("request to top up failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("response status not OK: %s", resp.Status)
	}

	return nil
}
