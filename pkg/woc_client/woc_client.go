package woc_client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"io"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	apiURL    = "https://api.whatsonchain.com/v1/bsv"
	maxIDsNum = 20 // value from doc
)

var (
	ErrWOCFailedToCreateRequest  = errors.New("failed to create request")
	ErrWOCFailedToGetUTXOs       = errors.New("failed to get utxos from WoC")
	ErrWOCRequestFailed          = errors.New("request to WoC failed")
	ErrWOCResponseNotOK          = errors.New("response status not OK")
	ErrWOCFailedToDecodeResponse = errors.New("failed to decode response")
	ErrWOCFailedToDecodeRawTxs   = errors.New("failed to decode raw txs")
	ErrWOCFailedToTopUp          = errors.New("top up can only be done on testnet")
)

type WocClient struct {
	client            http.Client
	authorization     string
	net               string
	logger            *slog.Logger
	url               string
	maxNumIDs         int
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

func WithTracer(attr ...attribute.KeyValue) func(s *WocClient) {
	return func(p *WocClient) {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}
	}
}

func WithLogger(logger *slog.Logger) func(*WocClient) {
	return func(p *WocClient) {
		p.logger = logger
	}
}

func WithURL(url string) func(*WocClient) {
	return func(p *WocClient) {
		p.url = url
	}
}

func WithMaxNumIDs(maxNumIDs int) func(*WocClient) {
	return func(p *WocClient) {
		p.maxNumIDs = maxNumIDs
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
		client:    http.Client{Timeout: 10 * time.Second},
		net:       net,
		url:       apiURL,
		maxNumIDs: maxIDsNum,
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
	Status   string `json:"status"`
}

type wocResponse struct {
	Address string    `json:"address"`
	Script  string    `json:"script"`
	Result  []wocUtxo `json:"result"`
}

type wocConfirmedBalance struct {
	Address   string `json:"address"`
	Script    string `json:"script"`
	Confirmed uint64 `json:"confirmed"`
	Error     string `json:"error"`
}

type wocUnconfirmedBalance struct {
	Address     string `json:"address"`
	Script      string `json:"script"`
	Unconfirmed uint64 `json:"unconfirmed"`
	Error       string `json:"error"`
}
type WocRawTx struct {
	TxID          string `json:"txid"`
	Hex           string `json:"hex"`
	BlockHash     string `json:"blockhash"`
	BlockHeight   uint64 `json:"blockheight"`
	BlockTime     uint64 `json:"blocktime"`
	Confirmations uint64 `json:"confirmations"`
	Error         string `json:"error"`
}

func (w *WocClient) GetUTXOsWithRetries(ctx context.Context, address string, constantBackoff time.Duration, retries uint64) (sdkTx.UTXOs, error) {
	policy := backoff.WithMaxRetries(backoff.NewConstantBackOff(constantBackoff), retries)

	policyContext := backoff.WithContext(policy, ctx)
	operation := func() (sdkTx.UTXOs, error) {
		wocUtxos, err := w.GetUTXOs(ctx, address)
		if err != nil {
			return nil, errors.Join(ErrWOCFailedToGetUTXOs, err)
		}
		return wocUtxos, nil
	}

	notify := func(err error, nextTry time.Duration) {
		w.logger.ErrorContext(ctx, "failed to get utxos from WoC", slog.String("address", address), slog.String("next try", nextTry.String()), slog.String("err", err.Error()))
	}

	utxos, err := backoff.RetryNotifyWithData(operation, policyContext, notify)
	if err != nil {
		return nil, err
	}
	return utxos, nil
}

// GetUTXOs Get UTXOs from WhatsOnChain
func (w *WocClient) GetUTXOs(ctx context.Context, address string) (sdkTx.UTXOs, error) {
	req, err := w.httpRequest(ctx, "GET", fmt.Sprintf("address/%s/unspent/all", address), nil)
	if err != nil {
		return nil, err
	}

	resp, err := w.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var wocUnspent *wocResponse
	err = json.NewDecoder(resp.Body).Decode(&wocUnspent)
	if err != nil {
		return nil, errors.Join(ErrWOCFailedToDecodeResponse, err)
	}

	unspent := make(sdkTx.UTXOs, len(wocUnspent.Result))
	wocScript, err := hex.DecodeString(wocUnspent.Script)
	if err != nil {
		return nil, errors.Join(ErrWOCFailedToDecodeResponse, err)
	}
	lockingScript := script.Script(wocScript)
	for i, utxo := range wocUnspent.Result {
		h, err := chainhash.NewHashFromHex(utxo.Txid)
		if err != nil {
			return unspent, err
		}
		unspent[i] = &sdkTx.UTXO{
			TxID:          h,
			Vout:          utxo.Vout,
			LockingScript: &lockingScript,
			Satoshis:      utxo.Satoshis,
		}
	}

	return unspent, nil
}

func (w *WocClient) GetBalance(ctx context.Context, address string) (uint64, uint64, error) {
	reqConfirmed, err := w.httpRequest(ctx, "GET", fmt.Sprintf("address/%s/confirmed/balance", address), nil)
	if err != nil {
		return 0, 0, err
	}

	resp, err := w.doRequest(reqConfirmed)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var confirmedBalance wocConfirmedBalance
	err = json.NewDecoder(resp.Body).Decode(&confirmedBalance)
	if err != nil {
		return 0, 0, errors.Join(ErrWOCFailedToDecodeResponse, err)
	}

	reqUnconfirmed, err := w.httpRequest(ctx, "GET", fmt.Sprintf("address/%s/unconfirmed/balance", address), nil)
	if err != nil {
		return 0, 0, err
	}

	resp2, err2 := w.doRequest(reqUnconfirmed)
	if err2 != nil {
		return 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var unconfirmedBalance wocUnconfirmedBalance
	err2 = json.NewDecoder(resp2.Body).Decode(&unconfirmedBalance)
	if err2 != nil {
		return 0, 0, errors.Join(ErrWOCFailedToDecodeResponse, err)
	}

	return confirmedBalance.Confirmed, unconfirmedBalance.Unconfirmed, nil
}

func (w *WocClient) GetBalanceWithRetries(ctx context.Context, address string, constantBackoff time.Duration, retries uint64) (uint64, uint64, error) {
	policy := backoff.WithMaxRetries(backoff.NewConstantBackOff(constantBackoff), retries)

	policyContext := backoff.WithContext(policy, ctx)

	type balanceResult struct {
		confirmed   uint64
		unconfirmed uint64
	}

	operation := func() (balanceResult, error) {
		confirmed, unconfirmed, err := w.GetBalance(ctx, address)
		if err != nil {
			return balanceResult{}, err
		}
		return balanceResult{confirmed: confirmed, unconfirmed: unconfirmed}, nil
	}

	notify := func(err error, nextTry time.Duration) {
		w.logger.ErrorContext(ctx, "failed to get balance from WoC", slog.String("address", address), slog.String("next try", nextTry.String()), slog.String("err", err.Error()))
	}

	balanceRes, err := backoff.RetryNotifyWithData(operation, policyContext, notify)
	if err != nil {
		return 0, 0, err
	}

	return balanceRes.confirmed, balanceRes.unconfirmed, nil
}

func (w *WocClient) TopUp(ctx context.Context, address string) error {
	if w.net != "test" {
		return ErrWOCFailedToTopUp
	}

	req, err := w.httpRequest(ctx, "GET", fmt.Sprintf("faucet/send/%s", address), nil)
	if err != nil {
		return err
	}

	_, err = w.doRequest(req)
	if err != nil {
		return err
	}

	return nil
}

func (w *WocClient) GetRawTxs(ctx context.Context, ids []string) (result []*WocRawTx, err error) {
	_, span := tracing.StartTracing(ctx, "WocClient_GetRawTxs", w.tracingEnabled, append(w.tracingAttributes, attribute.Int("ids", len(ids)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	for len(ids) > 0 {
		bsize := min(w.maxNumIDs, len(ids))
		batch := ids[:bsize]

		txs, err := w.getRawTxs(ctx, batch)
		if err != nil {
			return nil, err
		}

		result = append(result, txs...)
		ids = ids[bsize:] // move batch window

		time.Sleep(200 * time.Millisecond)
	}

	return result, nil
}

func (w *WocClient) getRawTxs(ctx context.Context, batch []string) (res []*WocRawTx, err error) {
	_, span := tracing.StartTracing(ctx, "WocClient_getRawTxs", w.tracingEnabled, append(w.tracingAttributes, attribute.Int("batch", len(batch)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()
	payload := map[string][]string{"txids": batch}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := w.httpRequest(ctx, "POST", "txs/hex", body)
	if err != nil {
		return nil, err
	}

	resp, err := w.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, errors.Join(ErrWOCFailedToDecodeRawTxs, err)
	}

	return res, nil
}

func (w *WocClient) httpRequest(ctx context.Context, method string, endpoint string, payload []byte) (*http.Request, error) {
	var body io.Reader
	if len(payload) > 0 {
		body = bytes.NewBuffer(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s/%s", w.url, w.net, endpoint), body)
	if err != nil {
		return nil, errors.Join(ErrWOCFailedToCreateRequest, err)
	}

	if w.authorization != "" {
		req.Header.Set("Authorization", w.authorization)
	}

	return req, nil
}

func (w *WocClient) doRequest(req *http.Request) (*http.Response, error) {
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, errors.Join(ErrWOCRequestFailed, err)
	}

	if resp.StatusCode != 200 {
		return nil, errors.Join(ErrWOCResponseNotOK, fmt.Errorf("status: %s", resp.Status))
	}

	return resp, nil
}
