package callbacker_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallbackSender_Send(t *testing.T) {
	tests := []struct {
		name            string
		responseStatus  int
		expectedSuccess bool
		expectedRetries int
	}{
		{
			name:            "success - callback sent",
			responseStatus:  http.StatusOK,
			expectedSuccess: true,
			expectedRetries: 1,
		},
		{
			name:            "retry - server error and fails after retries",
			responseStatus:  http.StatusInternalServerError,
			expectedSuccess: true,
			expectedRetries: 3, // Adjust based on your retry logic in `Send`
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			client := &http.Client{Timeout: 500 * time.Second}

			retries := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if retries < tc.expectedRetries-1 {
					w.WriteHeader(tc.responseStatus)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				retries++
				time.Sleep(100 * time.Millisecond)
			}))
			defer server.Close()

			logger := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
			sut, err := callbacker.NewSender(client, logger)
			require.NoError(t, err)

			defer sut.GracefulStop()

			//When
			success := sut.Send(server.URL, "test-token", &callbacker.Callback{TxID: "1234", TxStatus: "SEEN_ON_NETWORK"})
			//Then
			require.Equal(t, tc.expectedSuccess, success, "Expected success to be %v, but got %v", tc.expectedSuccess, success)
			require.Equal(t, tc.expectedRetries, retries, "Expected retries to be %d, but got %d", tc.expectedRetries, retries)
		})
	}
}
func TestCallbackSender_GracefulStop(t *testing.T) {
	// Given
	logger := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
	sut, err := callbacker.NewSender(http.DefaultClient, logger)
	require.NoError(t, err)

	// When: Call GracefulStop twice to ensure it handles double stop gracefully
	sut.GracefulStop()
	sut.GracefulStop()

	// Then: Assert that sender is disposed
	err = sut.Health()
	require.EqualError(t, err, "callbacker is disposed already")
}

func TestCallbackSender_Health(t *testing.T) {
	// Given
	logger := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
	sut, err := callbacker.NewSender(http.DefaultClient, logger)
	require.NoError(t, err)

	// When: Test Health on active sender
	err = sut.Health()

	// Then: Expect no error when sender is active
	require.NoError(t, err)

	// When: Stop sender and check Health error
	sut.GracefulStop()
	err = sut.Health()

	// Then: Expect error indicating the sender is disposed
	require.EqualError(t, err, "callbacker is disposed already")
}

func TestCallbackSender_SendBatch(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		token           string
		dtos            []*callbacker.Callback
		responseStatus  int
		expectedSuccess bool
		expectedRetries int
	}{
		{
			name:            "success - batch sent with retries",
			url:             "/batch",
			token:           "test-token",
			dtos:            []*callbacker.Callback{{TxID: "1234", TxStatus: "SEEN_ON_NETWORK"}, {TxID: "5678", TxStatus: "MINED"}},
			responseStatus:  http.StatusOK,
			expectedSuccess: true,
			expectedRetries: 1,
		},
		{
			name:            "failure - server error on batch with retries",
			url:             "/batch",
			token:           "test-token",
			dtos:            []*callbacker.Callback{{TxID: "1234", TxStatus: "SEEN_ON_NETWORK"}, {TxID: "5678", TxStatus: "MINED"}},
			responseStatus:  http.StatusInternalServerError,
			expectedSuccess: true,
			expectedRetries: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			retries := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				require.Equal(t, "Bearer "+tc.token, r.Header.Get("Authorization"))

				var batch callbacker.BatchCallback
				err := json.NewDecoder(r.Body).Decode(&batch)
				require.NoError(t, err)
				require.Equal(t, len(tc.dtos), batch.Count)

				if retries < tc.expectedRetries-1 {
					w.WriteHeader(tc.responseStatus)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				retries++
				time.Sleep(100 * time.Millisecond)
			}))
			defer server.Close()

			client := &http.Client{Timeout: 2 * time.Second}
			logger := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
			sut, err := callbacker.NewSender(client, logger)
			require.NoError(t, err)
			defer sut.GracefulStop()

			// When
			success := sut.SendBatch(server.URL+tc.url, tc.token, tc.dtos)

			// Then
			require.Equal(t, tc.expectedSuccess, success, "Expected success to be %v, but got %v", tc.expectedSuccess, success)
			require.Equal(t, tc.expectedRetries, retries, "Expected retries to be %d, but got %d", tc.expectedRetries, retries)
		})
	}
}
func TestCallbackSender_Send_WithRetries(t *testing.T) {
	// Given
	client := &http.Client{}
	logger := slog.New(slog.NewJSONHandler(bytes.NewBuffer([]byte{}), nil))
	sut, _ := callbacker.NewSender(client, logger)

	retryCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		retryCount++
		if retryCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// When
	callback := &callbacker.Callback{TxID: "test-txid", TxStatus: "SEEN_ON_NETWORK"}
	ok := sut.Send(server.URL, "test-token", callback)

	// Then
	assert.True(t, ok)
}
