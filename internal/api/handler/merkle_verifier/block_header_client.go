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

	"github.com/bitcoin-sv/arc/internal/api/handler"
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
	ErrGetChainTip     = errors.New("failed to get chain tip")
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

func WithStats(stats *handler.Stats) func(*Client) {
	return func(client *Client) {
		client.stats = stats
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
	stats                      *handler.Stats
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

func checkChainTrackers(ctx context.Context, c *Client) []attribute.KeyValue {
	availableChaintrackers := 0
	for _, ct := range c.chainTrackers {
		isAvailable, err := c.isServiceAvailable(ctx, ct.url, ct.apiKey)

		if err != nil {
			c.logger.Warn("Block header service unavailable", slog.String("url", ct.url), slog.Bool("isAvailable", isAvailable), slog.String("err", err.Error()))
		} else {
			availableChaintrackers++
			c.logger.Debug("Block header service available", slog.String("url", ct.url), slog.Bool("isAvailable", isAvailable))
		}

		ct.SetAvailability(isAvailable)
	}

	if c.stats != nil {
		c.stats.AvailableBlockHeaderServices.Set(float64(availableChaintrackers))
		c.stats.UnavailableBlockHeaderServices.Set(float64(len(c.chainTrackers) - availableChaintrackers))
	}
	return []attribute.KeyValue{}
}

func (c *Client) isServiceAvailable(ctx context.Context, url string, apiKey string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url+"/status", nil)
	if err != nil {
		return false, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
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

func (c *Client) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	var verificationSuccessful bool
	var anyChainTrackerAvailable bool
	var err error
	for _, ct := range c.chainTrackers {
		if !ct.IsAvailable() {
			continue
		}

		anyChainTrackerAvailable = true

		verificationSuccessful, err = c.merkleRootVerify(ctx, ct.url, ct.apiKey, root, height)
		if err == nil {
			break
		}
	}

	if !anyChainTrackerAvailable {
		return verificationSuccessful, errors.Join(beef.ErrNoChainTrackersAvailable, err)
	}

	return verificationSuccessful, err
}

type State struct {
	Height uint32 `json:"height"`
}

func (c *Client) getChainTip(ctx context.Context, url string, apiKey string) (uint32, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/v1/chain/tip/longest", url), nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		var e net.Error
		isNetError := errors.As(err, &e)
		if isNetError && e.Timeout() {
			return 0, errors.Join(ErrRequestTimedOut, err)
		}

		return 0, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, errors.Join(beef.ErrRequestFailed, fmt.Errorf("status code: %d, status: %s", resp.StatusCode, resp.Status))
	}

	var response State
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return 0, errors.Join(ErrParseResponse, fmt.Errorf("error unmarshaling JSON: %v", err))
	}
	return response.Height, nil
}

func (c *Client) CurrentHeight(ctx context.Context) (uint32, error) {
	var tip uint32
	var anyChainTrackerAvailable bool
	var err error
	for _, ct := range c.chainTrackers {
		if !ct.IsAvailable() {
			continue
		}

		anyChainTrackerAvailable = true
		tip, err = c.getChainTip(ctx, ct.url, ct.apiKey)
		if err == nil {
			break
		}
	}

	if err != nil {
		return 0, errors.Join(ErrGetChainTip, err)
	}

	if !anyChainTrackerAvailable {
		return 0, errors.Join(beef.ErrNoChainTrackersAvailable, err)
	}

	return tip, nil
}

type IsValidRootForHeightResponse struct {
	ConfirmationState bhsDomains.MerkleRootConfirmationState `json:"confirmationState"`
}

func (c *Client) merkleRootVerify(ctx context.Context, url string, apiKey string, root *chainhash.Hash, height uint32) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	type requestBody struct {
		MerkleRoot  string `json:"merkleRoot"`
		BlockHeight uint32 `json:"blockHeight"`
	}

	payload := []requestBody{{MerkleRoot: root.String(), BlockHeight: height}}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("error marshaling JSON: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url+"/api/v1/chain/merkleroot/verify", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return false, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
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
