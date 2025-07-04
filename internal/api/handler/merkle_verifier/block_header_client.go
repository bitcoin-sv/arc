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
)

const (
	checkChainTrackersIntervalDefault = 30 * time.Second
	statusTimeout                     = 500 * time.Millisecond
)

var (
	ErrRequestFailed   = errors.New("request failed")
	ErrRequestTimedOut = errors.New("request timed out")
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
	availability bool
	url          string
	apiKey       string
}

func (ct *ChainTracker) IsAvailable() bool {
	return ct.availability
}

func (ct *ChainTracker) SetAvailability(availability bool) {
	ct.availability = availability
}

func NewChainTracker(url string, apiKey string) *ChainTracker {
	return &ChainTracker{
		url:          url,
		apiKey:       apiKey,
		availability: true,
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

	mu            sync.RWMutex
	chainTrackers []*ChainTracker
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
			c.logger.Error("=== checkChainTrackers", "url", ct.url, "isAvailable", isAvailable, "err", err)
		} else {
			c.logger.Info("=== checkChainTrackers", "url", ct.url, "isAvailable", isAvailable)
		}

		c.mu.Lock()
		ct.SetAvailability(isAvailable)
		c.mu.Unlock()
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
	var err error
	for _, ct := range c.chainTrackers {
		if !ct.IsAvailable() {
			continue
		}

		verificationSuccessful, err = c.merkleRootVerify(ct.url, ct.apiKey, root, height)
		if err == nil {
			break
		}
	}

	return verificationSuccessful, err
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
		return false, fmt.Errorf("error reading response body: %v", err)
	}

	var response struct {
		ConfirmationState string `json:"confirmationState"`
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return false, fmt.Errorf("error unmarshaling JSON: %v", err)
	}

	return response.ConfirmationState == "CONFIRMED", nil
}

func (c *Client) Shutdown() {
	c.cancelAll()

	c.wg.Wait()
}
