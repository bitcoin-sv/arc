package metamorph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/ordishs/go-utils"
)

const (
	CallbackTries           = 5
	CallbackIntervalSeconds = 5
)

type Callback struct {
	BlockHash   *string   `json:"blockHash,omitempty"`
	BlockHeight *uint64   `json:"blockHeight,omitempty"`
	ExtraInfo   *string   `json:"extraInfo"`
	MerklePath  *string   `json:"merklePath"`
	Timestamp   time.Time `json:"timestamp"`
	TxStatus    *string   `json:"txStatus,omitempty"`
	Txid        string    `json:"txid"`
}

type Callbacker struct {
	httpClient HttpClient
	wg         sync.WaitGroup
	mu         sync.Mutex
	disposed   bool
}

func NewCallbacker(httpClient HttpClient) *Callbacker {
	return &Callbacker{
		httpClient: httpClient,
	}
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func (p *Callbacker) SendCallback(logger *slog.Logger, tx *store.StoreData) {
	p.mu.Lock()
	if p.disposed {
		logger.Error("cannot send callback, callbacker is disposed already")
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	p.wg.Add(1)
	defer p.wg.Done()

	sleepDuration := CallbackIntervalSeconds
	statusString := tx.Status.String()
	blockHash := ""
	if tx.BlockHash != nil {
		blockHash = utils.ReverseAndHexEncodeSlice(tx.BlockHash.CloneBytes())
	}

	for i := 0; i < CallbackTries; i++ {

		logger.Debug("Sending callback for transaction", slog.String("hash", tx.Hash.String()), slog.String("url", tx.CallbackUrl), slog.String("token", tx.CallbackToken), slog.String("status", statusString), slog.Uint64("block height", tx.BlockHeight), slog.String("block hash", blockHash))

		status := &Callback{
			BlockHash:   &blockHash,
			BlockHeight: &tx.BlockHeight,
			TxStatus:    &statusString,
			Txid:        tx.Hash.String(),
			Timestamp:   time.Now(),
			MerklePath:  &tx.MerklePath,
		}
		statusBytes, err := json.Marshal(status)
		if err != nil {
			logger.Error("Couldn't marshal status", slog.String("err", err.Error()))
			return
		}

		var request *http.Request
		request, err = http.NewRequest("POST", tx.CallbackUrl, bytes.NewBuffer(statusBytes))
		if err != nil {
			logger.Error("Couldn't marshal status", slog.String("url", tx.CallbackUrl), slog.String("token", tx.CallbackToken), slog.String("hash", tx.Hash.String()), slog.String("err", errors.Join(err, fmt.Errorf("failed to post callback for transaction id %s", tx.Hash)).Error()))
			return
		}
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		if tx.CallbackToken != "" {
			request.Header.Set("Authorization", "Bearer "+tx.CallbackToken)
		}

		var response *http.Response
		response, err = p.httpClient.Do(request)
		if err != nil {
			logger.Debug("Couldn't send transaction callback", slog.String("url", tx.CallbackUrl), slog.String("token", tx.CallbackToken), slog.String("hash", tx.Hash.String()), slog.String("err", err.Error()))
			continue
		}
		defer response.Body.Close()

		// if callback was sent successfully we stop here
		if response.StatusCode == http.StatusOK {
			return
		}

		logger.Debug("Callback response status code not ok", slog.String("url", tx.CallbackUrl), slog.String("token", tx.CallbackToken), slog.String("hash", tx.Hash.String()), slog.Int("status", response.StatusCode))

		// sleep before trying again
		time.Sleep(time.Duration(sleepDuration) * time.Second)
		// increase intervals on each failure
		sleepDuration *= 2
	}

	logger.Warn("Couldn't send transaction callback after tries", slog.String("url", tx.CallbackUrl), slog.String("token", tx.CallbackToken), slog.String("hash", tx.Hash.String()), slog.Int("retries", CallbackTries))
}

func (p *Callbacker) Shutdown(logger *slog.Logger) {
	p.mu.Lock()
	if p.disposed {
		logger.Info("callbacker is down already")
		p.mu.Unlock()
		return
	}

	p.disposed = true
	p.mu.Unlock()

	logger.Info("Shutting down callbacker")
	p.wg.Wait()
}
