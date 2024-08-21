package woc_client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/transaction"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type WocClient struct {
	client        http.Client
	authorization string
	net           string
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

func New(mainnet bool, opts ...func(client *WocClient)) *WocClient {
	net := "test"
	if mainnet {
		net = "main"
	}

	w := &WocClient{
		client: http.Client{Timeout: 10 * time.Second},
		net:    net,
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
	Error         string `json:"error"`
}

func (w *WocClient) GetUTXOsWithRetries(ctx context.Context, lockingScript *script.Script, address string, constantBackoff time.Duration, retries uint64) (transaction.UTXOs, error) {
	policy := backoff.WithMaxRetries(backoff.NewConstantBackOff(constantBackoff), retries)

	policyContext := backoff.WithContext(policy, ctx)

	operation := func() (transaction.UTXOs, error) {
		wocUtxos, err := w.GetUTXOs(ctx, lockingScript, address)
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

// GetUTXOs Get UTXOs from WhatsOnChain
func (w *WocClient) GetUTXOs(ctx context.Context, lockingScript *script.Script, address string) (transaction.UTXOs, error) {
	req, err := w.httpRequest(ctx, "GET", fmt.Sprintf("address/%s/unspent", address), nil)
	if err != nil {
		return nil, err
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

	unspent := make(transaction.UTXOs, len(wocUnspent))
	for i, utxo := range wocUnspent {
		txIDBytes, err := hex.DecodeString(utxo.Txid)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex string: %v", err)
		}

		unspent[i] = &transaction.UTXO{
			TxID:          txIDBytes,
			Vout:          utxo.Vout,
			LockingScript: lockingScript,
			Satoshis:      utxo.Satoshis,
		}
	}

	return unspent, nil
}

func (w *WocClient) GetBalance(ctx context.Context, address string) (int64, int64, error) {
	req, err := w.httpRequest(ctx, "GET", fmt.Sprintf("address/%s/balance", address), nil)
	if err != nil {
		return 0, 0, err
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

func (w *WocClient) GetBalanceWithRetries(ctx context.Context, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error) {
	policy := backoff.WithMaxRetries(backoff.NewConstantBackOff(constantBackoff), retries)

	policyContext := backoff.WithContext(policy, ctx)

	type balanceResult struct {
		confirmed   int64
		unconfirmed int64
	}

	operation := func() (balanceResult, error) {
		confirmed, unconfirmed, err := w.GetBalance(ctx, address)
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

func (w *WocClient) TopUp(ctx context.Context, address string) error {
	if w.net != "test" {
		return errors.New("top up can only be done on testnet")
	}

	req, err := w.httpRequest(ctx, "GET", fmt.Sprintf("faucet/send/%s", address), nil)
	if err != nil {
		return err
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

func (w *WocClient) GetRawTxs(ctx context.Context, ids []string) ([]*wocRawTx, error) {
	const maxIDsNum = 20 // value from doc

	var result []*wocRawTx

	for len(ids) > 0 {
		bsize := min(maxIDsNum, len(ids))
		batch := ids[:bsize]

		txs, err := w.getRawTxs(ctx, batch)
		if err != nil {
			return nil, err
		}

		result = append(result, txs...)
		ids = ids[bsize:] // move batch window
	}

	return result, nil
}

func (w *WocClient) getRawTxs(ctx context.Context, batch []string) ([]*wocRawTx, error) {
	payload := map[string][]string{"txids": batch}
	body, _ := json.Marshal(payload)

	req, err := w.httpRequest(ctx, "POST", "txs/hex", body)
	if err != nil {
		return nil, err
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

func (w WocClient) httpRequest(ctx context.Context, method string, endpoint string, payload []byte) (*http.Request, error) {
	const apiUrl = "https://api.whatsonchain.com/v1/bsv"

	var body io.Reader
	if len(payload) > 0 {
		body = bytes.NewBuffer(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s/%s", apiUrl, w.net, endpoint), body)
	if err != nil {
		return nil, fmt.Errorf("failed to crreate request: %v", err)
	}

	if w.authorization != "" {
		req.Header.Set("Authorization", w.authorization)
	}

	return req, nil
}
