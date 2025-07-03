package merkle_verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/validator/beef"
	"github.com/bitcoin-sv/arc/pkg/tracing"

	bhsDomains "github.com/bitcoin-sv/block-headers-service/domains"
)

const (
	checkChainTrackersIntervalDefault = 30 * time.Second
	statusTimeout                     = 500 * time.Millisecond
)

var (
	ErrRequestFailed   = errors.New("request failed")
	ErrRequestTimedOut = errors.New("request timed out")
	ErrParseResponse   = errors.New("failed to parse response")
)

type Option func(*Client)

func WithTimeout(timeout time.Duration) Option {
	return func(client *Client) {
		client.timeout = timeout
	}
}

func WithCheckChainTrackersInterval(d time.Duration) Option {
	return func(client *Client) {
		client.checkChainTrackersInterval = d
	}
}

type ChainTracker struct {
	url    string
	apiKey string

	mu          sync.Mutex
	isAvailable bool
}

func (ct *ChainTracker) IsAvailable() bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.isAvailable
}

func (ct *ChainTracker) SetAvailability(availability bool) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.isAvailable = availability
}

func NewChainTracker(url string, apiKey string) *ChainTracker {
	return &ChainTracker{
		url:         url,
		apiKey:      apiKey,
		isAvailable: true,
	}
}

type Client struct {
	logger                     *slog.Logger
	checkChainTrackersInterval time.Duration
	timeout                    time.Duration
	tracingEnabled             bool
	tracingAttributes          []attribute.KeyValue
	cancelAll                  context.CancelFunc
	ctx                        context.Context
	wg                         *sync.WaitGroup
	chainTrackers              []*ChainTracker
}

func NewClient(logger *slog.Logger, chainTrackers []*ChainTracker, opts ...Option) *Client {
	c := &Client{
		timeout:                    10 * time.Second,
		logger:                     logger,
		checkChainTrackersInterval: checkChainTrackersIntervalDefault,
		wg:                         &sync.WaitGroup{},
	}

	c.chainTrackers = chainTrackers

	ctx, cancelAll := context.WithCancel(context.Background())
	c.cancelAll = cancelAll
	c.ctx = ctx

	for _, opt := range opts {
		opt(c)
	}

	c.StartRoutine(c.checkChainTrackersInterval, checkChainTrackers, "checkChainTrackers")

	return c
}

func (c *Client) StartRoutine(tickerInterval time.Duration, routine func(context.Context, *Client) []attribute.KeyValue, routineName string) {
	ticker := time.NewTicker(tickerInterval)
	c.wg.Add(1)

	go func() {
		defer func() {
			c.wg.Done()
			ticker.Stop()
		}()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				ctx, span := tracing.StartTracing(c.ctx, routineName, c.tracingEnabled, c.tracingAttributes...)
				attr := routine(ctx, c)
				if span != nil && len(attr) > 0 {
					span.SetAttributes(attr...)
				}
				tracing.EndTracing(span, nil)
			}
		}
	}()
}

func checkChainTrackers(_ context.Context, c *Client) []attribute.KeyValue {
	for _, ct := range c.chainTrackers {
		isAvailable, err := c.isServiceAvailable(ct.url, ct.apiKey)

		if err != nil {
			c.logger.Warn("Block header service unavailable", slog.String("url", ct.url), slog.Bool("isAvailable", isAvailable), slog.String("err", err.Error()))
		} else {
			c.logger.Debug("Block header service available", slog.String("url", ct.url), slog.Bool("isAvailable", isAvailable))
		}

		ct.SetAvailability(isAvailable)
	}

	return []attribute.KeyValue{}
}

func (c *Client) isServiceAvailable(url string, apiKey string) (bool, error) {
	req, err := http.NewRequest("GET", url+"/status", nil)
	if err != nil {
		return false, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: statusTimeout}
	resp, err := client.Do(req)
	if err != nil {
		var e net.Error
		isNetError := errors.As(err, &e)
		if isNetError && e.Timeout() {
			return false, errors.Join(ErrRequestTimedOut, err)
		}

		return false, fmt.Errorf("error sending request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, errors.Join(ErrRequestFailed, fmt.Errorf("status code: %d, status: %s", resp.StatusCode, resp.Status))
	}

	return true, nil
}

func (c *Client) IsValidRootForHeight(root *chainhash.Hash, height uint32) (bool, error) {
	var verificationSuccessful bool
	var anyChainTrackerAvailable bool
	var err error
	for _, ct := range c.chainTrackers {
		if !ct.IsAvailable() {
			continue
		}

		anyChainTrackerAvailable = true

		verificationSuccessful, err = c.merkleRootVerify(ct.url, ct.apiKey, root, height)
		if err == nil {
			break
		}
	}

	if !anyChainTrackerAvailable {
		return verificationSuccessful, errors.Join(beef.ErrNoChainTrackersAvailable, err)
	}

	return verificationSuccessful, err
}

type IsValidRootForHeightResponse struct {
	ConfirmationState bhsDomains.MerkleRootConfirmationState `json:"confirmationState"`
}

func (c *Client) merkleRootVerify(url string, apiKey string, root *chainhash.Hash, height uint32) (bool, error) {
	type requestBody struct {
		MerkleRoot  string `json:"merkleRoot"`
		BlockHeight uint32 `json:"blockHeight"`
	}

	payload := []requestBody{{MerkleRoot: root.String(), BlockHeight: height}}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("error marshaling JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url+"/api/v1/chain/merkleroot/verify", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return false, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		var e net.Error
		isNetError := errors.As(err, &e)
		if isNetError && e.Timeout() {
			return false, errors.Join(beef.ErrRequestTimedOut, err)
		}

		return false, fmt.Errorf("error sending request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, errors.Join(beef.ErrRequestFailed, fmt.Errorf("status code: %d, status: %s", resp.StatusCode, resp.Status))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, errors.Join(ErrParseResponse, fmt.Errorf("error reading response body: %v", err))
	}

	var response IsValidRootForHeightResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return false, errors.Join(ErrParseResponse, fmt.Errorf("error unmarshaling JSON: %v", err))
	}

	if response.ConfirmationState != bhsDomains.Confirmed {
		c.logger.Warn("unconfirmed",
			slog.String("url", url),
			slog.String("root", root.String()),
			slog.Uint64("height", uint64(height)),
			slog.String("payload", string(jsonPayload)),
			slog.String("response", string(body)),
		)
	}

	return response.ConfirmationState == bhsDomains.Confirmed, nil
}

func (c *Client) Shutdown() {
	c.cancelAll()

	c.wg.Wait()
}
