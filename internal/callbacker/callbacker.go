package callbacker

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
)

const (
	retries                = 5
	initRetrySleepDuration = 5 * time.Second
)

type Callback struct {
	Timestamp time.Time `json:"timestamp"`

	CompetingTxs []string `json:"competingTxs,omitempty"`

	Txid       string  `json:"txid"`
	TxStatus   string  `json:"txStatus"`
	ExtraInfo  *string `json:"extraInfo,omitempty"`
	MerklePath *string `json:"merklePath,omitempty"`

	BlockHash   *string `json:"blockHash,omitempty"`
	BlockHeight *uint64 `json:"blockHeight,omitempty"`
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Callbacker struct {
	httpClient      HttpClient
	wg              sync.WaitGroup
	mu              sync.Mutex
	disposed        bool
	callbackerStats *stats
	logger          *slog.Logger
}

func New(httpClient HttpClient, logger *slog.Logger) (*Callbacker, error) {
	stats := NewCallbackerStats()

	err := registerStats(
		stats.callbackSeenOnNetworkCount,
		stats.callbackSeenInOrphanMempoolCount,
		stats.callbackDoubleSpendAttemptedCount,
		stats.callbackRejectedCount,
		stats.callbackMinedCount,
		stats.callbackFailedCount,
	)
	if err != nil {
		return nil, err
	}

	callbacker := &Callbacker{
		httpClient:      httpClient,
		callbackerStats: stats,
		logger:          logger.With(slog.String("module", "callbacker")),
	}

	return callbacker, nil
}

func (p *Callbacker) GracefulStop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.disposed {
		p.logger.Info("Callbacker is already stopped")
		return
	}

	p.logger.Info("Stopping callbacker")
	p.wg.Wait()

	unregisterStats(
		p.callbackerStats.callbackSeenOnNetworkCount,
		p.callbackerStats.callbackSeenInOrphanMempoolCount,
		p.callbackerStats.callbackDoubleSpendAttemptedCount,
		p.callbackerStats.callbackRejectedCount,
		p.callbackerStats.callbackMinedCount,
		p.callbackerStats.callbackFailedCount,
	)

	p.disposed = true
	p.logger.Info("Stopped Callbacker")
}

func (p *Callbacker) Health() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.disposed {
		return errors.New("callbacker is disposed already")
	}

	return nil
}

func (p *Callbacker) Send(url, token string, dto *Callback) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		ok := p.sendCallbackWithRetries(url, token, dto)

		if ok {
			p.updateSuccessStats(dto.TxStatus)
		} else {
			p.logger.Warn("Couldn't send transaction callback after retries",
				slog.String("url", url),
				slog.String("token", token),
				slog.String("hash", dto.Txid),
				slog.Int("retries", retries))

			p.callbackerStats.callbackFailedCount.Inc()
		}
	}()
}

func (p *Callbacker) sendCallbackWithRetries(url, token string, dto *Callback) bool {
	retrySleep := initRetrySleepDuration
	ok, retry := false, false
	for range retries {
		ok, retry = p.sendCallback(url, token, dto)

		// break on success or on non-retryable error (e.g., invalid URL)
		if ok || !retry {
			break
		}

		time.Sleep(retrySleep)
		// increase intervals on each failure
		retrySleep *= 2
	}
	return ok
}

func (p *Callbacker) sendCallback(url, token string, dto *Callback) (ok, retry bool) {
	statusBytes, err := json.Marshal(dto)
	if err != nil {
		p.logger.Error("Couldn't marshal status", slog.String("err", err.Error()))
		return false, false
	}

	request, err := httpRequest(url, token, statusBytes)
	if err != nil {
		p.logger.Error("Couldn't create HTTP request",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("hash", dto.Txid),
			slog.String("err", err.Error()))
		return false, false
	}

	response, err := p.httpClient.Do(request)
	if err != nil {
		p.logger.Warn("Couldn't send transaction callback",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("hash", dto.Txid),
			slog.String("err", err.Error()))
		return false, true
	}
	defer response.Body.Close()

	ok = response.StatusCode >= http.StatusOK && response.StatusCode < 300
	return ok, true // return 'retry' true in case statuscode was not a success code
}

func (p *Callbacker) updateSuccessStats(txStatus string) {
	status, ok := callbacker_api.Status_value[txStatus]
	if ok {
		switch callbacker_api.Status(status) {
		case callbacker_api.Status_SEEN_ON_NETWORK:
			p.callbackerStats.callbackSeenOnNetworkCount.Inc()
		case callbacker_api.Status_SEEN_IN_ORPHAN_MEMPOOL:
			p.callbackerStats.callbackSeenInOrphanMempoolCount.Inc()
		case callbacker_api.Status_DOUBLE_SPEND_ATTEMPTED:
			p.callbackerStats.callbackDoubleSpendAttemptedCount.Inc()
		case callbacker_api.Status_MINED:
			p.callbackerStats.callbackMinedCount.Inc()
		case callbacker_api.Status_REJECTED:
			p.callbackerStats.callbackRejectedCount.Inc()
		}
	}
}

func httpRequest(url, token string, payload []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req, nil
}
