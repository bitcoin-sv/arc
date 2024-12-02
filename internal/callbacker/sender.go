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

type CallbackSender struct {
	httpClient             *http.Client
	mu                     sync.Mutex
	disposed               bool
	stats                  *stats
	logger                 *slog.Logger
	retries                int
	initRetrySleepDuration time.Duration
}

type SenderOption func(s *CallbackSender)

func WithInitRetrySleepDuration(d time.Duration) func(*CallbackSender) {
	return func(s *CallbackSender) {
		s.initRetrySleepDuration = d
	}
}

func WithRetries(d int) func(*CallbackSender) {
	return func(s *CallbackSender) {
		s.retries = d
	}
}

func NewSender(httpClient *http.Client, logger *slog.Logger, opts ...SenderOption) (*CallbackSender, error) {
	stats := newCallbackerStats()

	err := registerStats(
		stats.callbackSeenOnNetworkCount,
		stats.callbackSeenInOrphanMempoolCount,
		stats.callbackDoubleSpendAttemptedCount,
		stats.callbackRejectedCount,
		stats.callbackMinedCount,
		stats.callbackFailedCount,
		stats.callbackBatchCount,
	)
	if err != nil {
		return nil, err
	}

	callbacker := &CallbackSender{
		httpClient:             httpClient,
		stats:                  stats,
		logger:                 logger.With(slog.String("module", "sender")),
		retries:                5,
		initRetrySleepDuration: 5 * time.Second,
	}

	// apply options to processor
	for _, opt := range opts {
		opt(callbacker)
	}

	return callbacker, nil
}

func (p *CallbackSender) GracefulStop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.disposed {
		p.logger.Info("Sender is already stopped")
		return
	}

	p.logger.Info("Stopping Sender")

	unregisterStats(
		p.stats.callbackSeenOnNetworkCount,
		p.stats.callbackSeenInOrphanMempoolCount,
		p.stats.callbackDoubleSpendAttemptedCount,
		p.stats.callbackRejectedCount,
		p.stats.callbackMinedCount,
		p.stats.callbackFailedCount,
		p.stats.callbackBatchCount,
	)

	p.disposed = true
	p.logger.Info("Stopped Sender")
}

func (p *CallbackSender) Health() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.disposed {
		return errors.New("callbacker is disposed already")
	}

	return nil
}

func (p *CallbackSender) Send(url, token string, dto *Callback) (ok bool) {
	payload, err := json.Marshal(dto)
	if err != nil {
		p.logger.Error("Couldn't marshal callback",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("hash", dto.TxID),
			slog.String("status", dto.TxStatus),
			slog.String("err", err.Error()))
		return false
	}
	var retries int
	ok, retries = p.sendCallbackWithRetries(url, token, payload)

	if ok {
		p.logger.Info("Callback sent",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("hash", dto.TxID),
			slog.String("status", dto.TxStatus),
			slog.Int("retries", retries))

		p.updateSuccessStats(dto.TxStatus)
		return
	}

	p.logger.Warn("Couldn't send transaction callback after retries",
		slog.String("url", url),
		slog.String("token", token),
		slog.String("hash", dto.TxID),
		slog.String("status", dto.TxStatus),
		slog.Int("retries", retries))

	p.stats.callbackFailedCount.Inc()
	return
}

func (p *CallbackSender) SendBatch(url, token string, dtos []*Callback) (ok bool) {
	batch := BatchCallback{
		Count:     len(dtos),
		Callbacks: dtos,
	}

	payload, err := json.Marshal(batch)
	if err != nil {
		p.logger.Error("Couldn't marshal callback",
			slog.String("url", url),
			slog.String("token", token),
			slog.Bool("batch", true),
			slog.String("err", err.Error()))

		return false
	}
	var retries int
	ok, retries = p.sendCallbackWithRetries(url, token, payload)
	p.stats.callbackBatchCount.Inc()
	if ok {
		for _, c := range dtos {
			p.logger.Info("Callback sent",
				slog.String("url", url),
				slog.String("token", token),
				slog.String("hash", c.TxID),
				slog.String("status", c.TxStatus),
				slog.Int("retries", retries))

			p.updateSuccessStats(c.TxStatus)
		}
		return
	}

	p.logger.Warn("Couldn't send transaction callback after retries",
		slog.String("url", url),
		slog.String("token", token),
		slog.Bool("batch", true),
		slog.Int("retries", retries))

	p.stats.callbackFailedCount.Inc()
	return
}

func (p *CallbackSender) sendCallbackWithRetries(url, token string, jsonPayload []byte) (ok bool, nrOfRetries int) {
	retrySleep := p.initRetrySleepDuration
	ok, retry := false, false
	counter := 0
	for range p.retries {
		counter++
		ok, retry = p.sendCallback(url, token, jsonPayload)

		// break on success or on non-retryable error (e.g., invalid URL)
		if ok || !retry {
			break
		}

		time.Sleep(retrySleep)
		// increase intervals on each failure
		retrySleep *= 2
	}
	return ok, counter
}

func (p *CallbackSender) sendCallback(url, token string, payload []byte) (ok, retry bool) {
	request, err := httpRequest(url, token, payload)
	if err != nil {
		p.logger.Error("Couldn't create HTTP request",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("err", err.Error()))
		return false, false
	}

	response, err := p.httpClient.Do(request)
	if err != nil {
		p.logger.Warn("Couldn't send transaction callback",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("err", err.Error()))
		return false, true
	}
	defer response.Body.Close()

	ok = response.StatusCode >= http.StatusOK && response.StatusCode < 300
	retry = !ok
	return
}

func (p *CallbackSender) updateSuccessStats(txStatus string) {
	status, ok := callbacker_api.Status_value[txStatus]
	if ok {
		switch callbacker_api.Status(status) {
		case callbacker_api.Status_SEEN_ON_NETWORK:
			p.stats.callbackSeenOnNetworkCount.Inc()
		case callbacker_api.Status_SEEN_IN_ORPHAN_MEMPOOL:
			p.stats.callbackSeenInOrphanMempoolCount.Inc()
		case callbacker_api.Status_DOUBLE_SPEND_ATTEMPTED:
			p.stats.callbackDoubleSpendAttemptedCount.Inc()
		case callbacker_api.Status_MINED:
			p.stats.callbackMinedCount.Inc()
		case callbacker_api.Status_REJECTED:
			p.stats.callbackRejectedCount.Inc()
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
