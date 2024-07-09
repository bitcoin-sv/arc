package woc_client

import (
	"bytes"
	"container/list"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

type WocClient struct {
	client        http.Client
	authorization string
	logger        *slog.Logger
}

func WithLogger(logger *slog.Logger) func(*WocClient) {
	return func(p *WocClient) {
		p.logger = logger
	}
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

type wocRawTx struct {
	TxID          string `json:"txid"`
	Hex           string `json:"hex"`
	BlockHash     string `json:"blockhash"`
	BlockHeight   uint64 `json:"blockheight"`
	BlockTime     uint64 `json:"blocktime"`
	Confirmations uint64 `json:"confirmations"`
}

func (w *WocClient) GetUTXOsWithRetries(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) ([]*bt.UTXO, error) {
	policy := backoff.WithMaxRetries(backoff.NewConstantBackOff(constantBackoff), retries)

	policyContext := backoff.WithContext(policy, ctx)

	operation := func() ([]*bt.UTXO, error) {
		wocUtxos, err := w.getUTXOs(ctx, mainnet, lockingScript, address)
		if err != nil {
			return nil, fmt.Errorf("failed to get utxos from WoC: %v", err)
		}
		return wocUtxos, nil
	}

	notify := func(err error, nextTry time.Duration) {
		w.logger.Error("failed to get utxos from WoC", slog.String("address", address), slog.String("next try", nextTry.String()), slog.String("err", err.Error()))
	}

	utxos, err := backoff.RetryNotifyWithData(operation, policyContext, notify)
	if err != nil {
		return nil, err
	}

	return utxos, nil
}

func (w *WocClient) getUTXOs(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error) {
	net := "test"
	if mainnet {
		net = "main"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent", net, address), nil)
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

// GetUTXOs Get UTXOs from WhatsOnChain
func (w *WocClient) GetUTXOs(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error) {
	return w.getUTXOs(ctx, mainnet, lockingScript, address)
}

func (w *WocClient) getUTXOsList(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) (*list.List, error) {
	utxos, err := w.getUTXOs(ctx, mainnet, lockingScript, address)
	if err != nil {
		return nil, err
	}

	values := list.New()
	for _, utxo := range utxos {
		values.PushBack(utxo)
	}

	return values, nil
}

// GetUTXOsList Get UTXOs from WhatsOnChain as a list
func (w *WocClient) GetUTXOsList(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) (*list.List, error) {
	return w.getUTXOsList(ctx, mainnet, lockingScript, address)
}

func (w *WocClient) GetUTXOsListWithRetries(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) (*list.List, error) {
	policy := backoff.WithMaxRetries(backoff.NewConstantBackOff(constantBackoff), retries)

	policyContext := backoff.WithContext(policy, ctx)

	operation := func() (*list.List, error) {
		wocUtxos, err := w.getUTXOsList(ctx, mainnet, lockingScript, address)
		if err != nil {
			return nil, fmt.Errorf("failed to get utxos from WoC: %v", err)
		}
		return wocUtxos, nil
	}

	notify := func(err error, nextTry time.Duration) {
		w.logger.Error("failed to get utxos from WoC", slog.String("address", address), slog.String("next try", nextTry.String()), slog.String("err", err.Error()))
	}

	utxos, err := backoff.RetryNotifyWithData(operation, policyContext, notify)
	if err != nil {
		return nil, err
	}

	return utxos, nil
}

func (w *WocClient) getBalance(ctx context.Context, mainnet bool, address string) (int64, int64, error) {
	net := "test"
	if mainnet {
		net = "main"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/balance", net, address), nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to crreate request: %v", err)
	}

	if w.authorization != "" {
		req.Header.Set("Authorization", w.authorization)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("request to get balance failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("response status not OK: %s", resp.Status)
	}

	var balance wocBalance
	err = json.NewDecoder(resp.Body).Decode(&balance)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode response: %v", err)
	}

	return balance.Confirmed, balance.Unconfirmed, nil
}

func (w *WocClient) GetBalance(ctx context.Context, mainnet bool, address string) (int64, int64, error) {
	return w.getBalance(ctx, mainnet, address)
}

func (w *WocClient) GetBalanceWithRetries(ctx context.Context, mainnet bool, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error) {
	policy := backoff.WithMaxRetries(backoff.NewConstantBackOff(constantBackoff), retries)

	policyContext := backoff.WithContext(policy, ctx)

	type balanceResult struct {
		confirmed   int64
		unconfirmed int64
	}

	operation := func() (balanceResult, error) {
		confirmed, unconfirmed, err := w.getBalance(ctx, mainnet, address)
		if err != nil {
			return balanceResult{}, fmt.Errorf("failed to get utxos from WoC: %v", err)
		}
		return balanceResult{confirmed: confirmed, unconfirmed: unconfirmed}, nil
	}

	notify := func(err error, nextTry time.Duration) {
		w.logger.Error("failed to get utxos from WoC", slog.String("address", address), slog.String("next try", nextTry.String()), slog.String("err", err.Error()))
	}

	balanceRes, err := backoff.RetryNotifyWithData(operation, policyContext, notify)
	if err != nil {
		return 0, 0, err
	}

	return balanceRes.confirmed, balanceRes.unconfirmed, nil
}

func (w *WocClient) TopUp(ctx context.Context, mainnet bool, address string) error {
	net := "test"
	if mainnet {
		return errors.New("top up can only be done on testnet")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api-test.whatsonchain.com/v1/bsv/%s/faucet/send/%s", net, address), nil)
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

func (w *WocClient) GetRawTxs(ctx context.Context, mainnet bool, ids []string) ([]*wocRawTx, error) {
	const maxIDsNum = 20 // value from doc

	var result []*wocRawTx

	net := "test"
	if mainnet {
		net = "main"
	}

	for len(ids) > 0 {
		bsize := min(maxIDsNum, len(ids))
		batch := ids[:bsize]

		txs, err := w.getRawTxs(ctx, net, batch)
		if err != nil {
			return nil, err
		}

		result = append(result, txs...)
		ids = ids[bsize:] // move batch window
	}

	return result, nil
}

func (w *WocClient) getRawTxs(ctx context.Context, net string, batch []string) ([]*wocRawTx, error) {
	payload := map[string][]string{"txs": batch}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/txs/hex", net), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to crreate request: %v", err)
	}

	if w.authorization != "" {
		req.Header.Set("Authorization", w.authorization)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to get balance failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("response status not OK: %s", resp.Status)
	}

	var res []*wocRawTx
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return res, nil
}
