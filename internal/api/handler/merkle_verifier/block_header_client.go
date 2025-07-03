package merkle_verifier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"

	"github.com/bitcoin-sv/arc/internal/validator/beef"
)

type Option func(*Client)

func WithTimeout(timeout time.Duration) Option {
	return func(client *Client) {
		client.timeout = timeout
	}
}

func WithAPIKey(apiKey string) Option {
	return func(client *Client) {
		client.apiKey = apiKey
	}
}

func NewClient(url string, opts ...Option) *Client {
	c := &Client{
		url:     url,
		timeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type Client struct {
	url     string
	apiKey  string
	timeout time.Duration
}

func (c Client) IsValidRootForHeight(root *chainhash.Hash, height uint32) (bool, error) {
	type requestBody struct {
		MerkleRoot  string `json:"merkleRoot"`
		BlockHeight uint32 `json:"blockHeight"`
	}

	payload := []requestBody{{MerkleRoot: root.String(), BlockHeight: height}}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("error marshaling JSON: %v", err)
	}

	req, err := http.NewRequest("POST", c.url+"/api/v1/chain/merkleroot/verify", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return false, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		var e net.Error
		isNetError := errors.As(err, &e)
		if isNetError && e.Timeout() {
			// if timeout was reached try next chain tracker
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
