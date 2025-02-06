package callbacker

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
)

type CallbackSender struct {
	httpClient         *http.Client
	mu                 sync.Mutex
	disposed           bool
	stats              *stats
	logger             *slog.Logger
	retries            int
	retrySleepDuration time.Duration
}

type SenderOption func(s *CallbackSender)

const (
	retriesDefault                = 5
	initRetrySleepDurationDefault = 5 * time.Second
)

func WithInitRetrySleepDuration(d time.Duration) func(*CallbackSender) {
	return func(s *CallbackSender) {
		s.retrySleepDuration = d
	}
}

func WithRetries(d int) func(*CallbackSender) {
	return func(s *CallbackSender) {
		s.retries = d
	}
}

func NewSender(httpClient *http.Client, logger *slog.Logger, opts ...SenderOption) (*CallbackSender, error) {
	cbStats := newCallbackerStats()

	err := registerStats(
		cbStats.callbackSeenOnNetworkCount,
		cbStats.callbackSeenInOrphanMempoolCount,
		cbStats.callbackDoubleSpendAttemptedCount,
		cbStats.callbackRejectedCount,
		cbStats.callbackMinedCount,
		cbStats.callbackFailedCount,
		cbStats.callbackBatchCount,
	)
	if err != nil {
		return nil, err
	}

	callbacker := &CallbackSender{
		httpClient:         httpClient,
		stats:              cbStats,
		logger:             logger.With(slog.String("module", "sender")),
		retries:            retriesDefault,
		retrySleepDuration: 5 * time.Second,
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

func (p *CallbackSender) Send(url, token string, dto *Callback) (success, retry bool) {
	payload, err := json.Marshal(dto)
	if err != nil {
		p.logger.Error("Failed to marshal callback",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("hash", dto.TxID),
			slog.String("status", dto.TxStatus),
			slog.String("timestamp", dto.Timestamp.String()),
			slog.String("err", err.Error()))
		return false, false
	}
	var retries int
	success, retry, retries = p.sendCallbackWithRetries(url, token, payload)

	if success {
		p.logger.Info("Callback sent",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("hash", dto.TxID),
			slog.String("status", dto.TxStatus),
			slog.String("timestamp", dto.Timestamp.String()),
			slog.Int("retries", retries),
		)

		p.updateSuccessStats(dto.TxStatus)
		return success, retry
	}

	p.logger.Warn("Failed to send callback with retries",
		slog.String("url", url),
		slog.String("token", token),
		slog.String("hash", dto.TxID),
		slog.String("status", dto.TxStatus),
		slog.String("timestamp", dto.Timestamp.String()),
		slog.Int("retries", retries),
	)

	p.stats.callbackFailedCount.Inc()
	return success, retry
}

func (p *CallbackSender) SendBatch(url, token string, dtos []*Callback) (success, retry bool) {
	batch := BatchCallback{
		Count:     len(dtos),
		Callbacks: dtos,
	}

	payload, err := json.Marshal(batch)
	if err != nil {
		p.logger.Error("Failed to marshal callback",
			slog.String("url", url),
			slog.String("token", token),
			slog.Bool("batch", true),
			slog.String("err", err.Error()))

		return false, false
	}
	var retries int
	success, retry, retries = p.sendCallbackWithRetries(url, token, payload)
	p.stats.callbackBatchCount.Inc()
	if success {
		for _, dto := range dtos {
			p.logger.Info("Callback sent in batch",
				slog.String("url", url),
				slog.String("token", token),
				slog.String("hash", dto.TxID),
				slog.String("status", dto.TxStatus),
				slog.String("timestamp", dto.Timestamp.String()),
				slog.Int("retries", retries),
			)

			p.updateSuccessStats(dto.TxStatus)
		}
		return success, retry
	}

	p.logger.Warn("Failed to send callback with retries",
		slog.String("url", url),
		slog.String("token", token),
		slog.Bool("batch", true),
		slog.Int("retries", retries))

	p.stats.callbackFailedCount.Inc()
	return success, retry
}

func (p *CallbackSender) sendCallbackWithRetries(url, token string, jsonPayload []byte) (success bool, retry bool, nrOfRetries int) {
	retrySleep := p.retrySleepDuration
	var err error
	var statusCode int

	retry = true
	for range p.retries {
		nrOfRetries++
		statusCode, err = p.sendCallback(url, token, jsonPayload)
		if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
			success = true
			retry = false
			break
		}

		if err != nil {
			if errors.Is(err, ErrCreateHTTPRequestFailed) {
				p.logger.Error("Failed to create HTTP request",
					slog.String("url", url),
					slog.String("token", token),
					slog.String("err", err.Error()))
				return false, true, nrOfRetries
			}

			if errors.Is(err, ErrHostNonExistent) {
				p.logger.Warn("Host does not exist",
					slog.String("url", url),
					slog.String("token", token),
					slog.String("err", err.Error()))
				return false, false, nrOfRetries
			}

			if errors.Is(err, ErrHTTPSendFailed) {
				p.logger.Warn("Failed to send callback",
					slog.String("url", url),
					slog.String("token", token),
					slog.String("err", err.Error()))

				time.Sleep(retrySleep)
				continue
			}
		}

		p.logger.Warn("Callback response not successful",
			slog.String("url", url),
			slog.String("token", token),
			slog.Int("status", statusCode))

		time.Sleep(retrySleep)
	}

	return success, retry, nrOfRetries
}

var (
	ErrHostNonExistent         = errors.New("host non existent")
	ErrCreateHTTPRequestFailed = errors.New("failed to create http request")
	ErrHTTPSendFailed          = errors.New("failed to send http request")
)

func (p *CallbackSender) sendCallback(url, token string, payload []byte) (statusCode int, err error) {
	request, err := httpRequest(url, token, payload)
	if err != nil {
		return 0, errors.Join(ErrCreateHTTPRequestFailed, err)
	}

	response, err := p.httpClient.Do(request)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") {
			return 0, errors.Join(ErrHostNonExistent, err)
		}
		return 0, errors.Join(ErrHTTPSendFailed, err)
	}
	defer response.Body.Close()

	return response.StatusCode, nil
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
