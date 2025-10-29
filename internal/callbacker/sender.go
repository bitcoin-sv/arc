package callbacker

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

type CallbackSender struct {
	mu       sync.Mutex
	disposed bool
	logger   *slog.Logger
	timeout  time.Duration
}

type SenderOption func(s *CallbackSender)

const (
	timeoutDefault = 5 * time.Second
)

func WithTimeout(d time.Duration) func(*CallbackSender) {
	return func(s *CallbackSender) {
		s.timeout = d
	}
}

func NewSender(logger *slog.Logger, opts ...SenderOption) (*CallbackSender, error) {
	callbacker := &CallbackSender{
		logger:  logger.With(slog.String("module", "sender")),
		timeout: timeoutDefault,
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
	success, retry = sendCallback(url, token, dto, p.logger.With(slog.String("hash", dto.TxID), slog.String("status", dto.TxStatus)), p.timeout)
	if success {
		return success, retry
	}

	return success, retry
}

func sendCallback(url, token string, dto *Callback, logger *slog.Logger, timeout time.Duration) (success, retry bool) {
	var err error
	var statusCode int
	var responseText string
	success = false
	retry = true

	jsonPayload, err := json.Marshal(dto)
	if err != nil {
		retry = false

		logger.Error("Failed to marshal callback",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("hash", dto.TxID),
			slog.String("status", dto.TxStatus),
			slog.String("timestamp", dto.Timestamp.String()),
			slog.String("err", err.Error()))
		return
	}

	statusCode, responseText, err = sendPayload(url, token, jsonPayload, timeout)
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		logger.Info("Callback sent",
			slog.String("url", url),
			slog.String("token", token),
			slog.String("hash", dto.TxID),
			slog.String("status", dto.TxStatus),
			slog.String("timestamp", dto.Timestamp.String()),
		)
		success = true
		retry = false
		return
	}

	if err != nil {
		if errors.Is(err, ErrHostNonExistent) {
			retry = false
		}
		logger.Error("Failed to send payload",
			slog.String("url", url),
			slog.String("token", token),
			slog.Int("code", statusCode),
			slog.String("resp", responseText),
			slog.String("err", err.Error()))
		return
	}
	logger.Warn("Failed to send payload - http status code is not OK",
		slog.String("url", url),
		slog.String("token", token),
		slog.String("hash", dto.TxID),
		slog.String("status", dto.TxStatus),
		slog.String("timestamp", dto.Timestamp.String()),
		slog.String("resp", responseText),
		slog.Int("code", statusCode),
		slog.Bool("batch", false),
	)

	return
}

func (p *CallbackSender) SendBatch(url, token string, dtos []*Callback) (success, retry bool) {
	success, retry = sendBatchCallback(url, token, dtos, p.logger.With(slog.Int("batch size", len(dtos))), p.timeout)
	if success {
		return success, retry
	}

	return success, retry
}

func sendBatchCallback(url, token string, dtos []*Callback, logger *slog.Logger, timeout time.Duration) (success, retry bool) {
	var err error
	var statusCode int
	var responseText string
	success = false
	retry = true

	batch := BatchCallback{
		Count:     len(dtos),
		Callbacks: dtos,
	}

	jsonPayload, err := json.Marshal(batch)
	if err != nil {
		logger.Error("Failed to marshal callback",
			slog.String("url", url),
			slog.String("token", token),
			slog.Bool("batch", true),
			slog.String("err", err.Error()))

		return false, false
	}

	statusCode, responseText, err = sendPayload(url, token, jsonPayload, timeout)
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		for _, dto := range dtos {
			logger.Info("Callback sent in batch",
				slog.String("url", url),
				slog.String("token", token),
				slog.String("hash", dto.TxID),
				slog.String("status", dto.TxStatus),
				slog.String("timestamp", dto.Timestamp.String()),
				slog.Int("batch size", len(dtos)),
			)
		}
		success = true
		retry = false
		return
	}

	if err != nil {
		if errors.Is(err, ErrHostNonExistent) {
			retry = false
		}
		logger.Error("Failed to send payload",
			slog.String("url", url),
			slog.String("token", token),
			slog.Int("code", statusCode),
			slog.String("resp", responseText),
			slog.String("err", err.Error()))
		return
	}

	logger.Warn("Failed to send payload - http status code is not OK",
		slog.String("url", url),
		slog.String("token", token),
		slog.String("resp", responseText),
		slog.Int("code", statusCode),
		slog.Bool("batch", true),
	)

	return
}

var (
	ErrHostNonExistent         = errors.New("host non existent")
	ErrCreateHTTPRequestFailed = errors.New("failed to create http request")
	ErrHTTPSendFailed          = errors.New("failed to send http request")
)

func sendPayload(url, token string, payload []byte, timeout time.Duration) (statusCode int, responseText string, err error) {
	request, err := httpRequest(url, token, payload)
	if err != nil {
		return 0, responseText, errors.Join(ErrCreateHTTPRequestFailed, err)
	}

	httpClient := &http.Client{Timeout: timeout}

	response, err := httpClient.Do(request)
	if err != nil {
		var e net.Error
		isNetError := errors.As(err, &e)
		if isNetError {
			return 0, responseText, errors.Join(ErrHostNonExistent, err)
		}
		return 0, responseText, errors.Join(ErrHTTPSendFailed, err)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusMultipleChoices {
		responseBody, err := io.ReadAll(response.Body)

		if err == nil {
			responseText = string(responseBody)
		}
	}

	return response.StatusCode, responseText, nil
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
