package callbacker_test

import (
	"bytes"
	"encoding/json"
	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			retries := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.responseStatus)
				retries++
			}))
			defer server.Close()
			logger := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
			sender, err := callbacker.NewSender(server.Client(), logger)
			require.NoError(t, err)
			defer sender.GracefulStop()

			// When
			success := sender.Send(server.URL, "test-token", &callbacker.Callback{TxID: "1234", TxStatus: "SEEN_ON_NETWORK"})

			// Then
			require.Equal(t, tc.expectedSuccess, success)
			require.Equal(t, tc.expectedRetries, retries)
		})
	}
}

func TestCallbackSender_GracefulStop(t *testing.T) {
	// Given
	logger := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
	sender, err := callbacker.NewSender(http.DefaultClient, logger)
	require.NoError(t, err)

	// When: Call GracefulStop twice to ensure it handles double stop gracefully
	sender.GracefulStop()
	sender.GracefulStop()

	// Then: Assert that sender is disposed
	err = sender.Health()
	require.EqualError(t, err, "callbacker is disposed already")
}

func TestCallbackSender_Health(t *testing.T) {
	// Given
	logger := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
	sender, err := callbacker.NewSender(http.DefaultClient, logger)
	require.NoError(t, err)

	// When: Test Health on active sender
	err = sender.Health()

	// Then: Expect no error when sender is active
	require.NoError(t, err)

	// When: Stop sender and check Health error
	sender.GracefulStop()
	err = sender.Health()

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
	}{
		{
			name:            "success - batch sent",
			url:             "/batch",
			token:           "test-token",
			dtos:            []*callbacker.Callback{{TxID: "1234", TxStatus: "SEEN_ON_NETWORK"}, {TxID: "5678", TxStatus: "MINED"}},
			responseStatus:  http.StatusOK,
			expectedSuccess: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer "+tc.token, r.Header.Get("Authorization"))

				var batch callbacker.BatchCallback
				err := json.NewDecoder(r.Body).Decode(&batch)
				require.NoError(t, err)
				require.Equal(t, len(tc.dtos), batch.Count)

				w.WriteHeader(tc.responseStatus)
			}))
			defer server.Close()

			logger := slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))
			sender, err := callbacker.NewSender(server.Client(), logger)
			require.NoError(t, err)
			defer sender.GracefulStop()

			// When
			success := sender.SendBatch(server.URL+tc.url, tc.token, tc.dtos)

			// Then
			require.Equal(t, tc.expectedSuccess, success)
		})
	}
}

func TestCallbackSender_Send_WithRetries(t *testing.T) {
	// Given
	client := &http.Client{}
	logger := slog.New(slog.NewJSONHandler(bytes.NewBuffer([]byte{}), nil))
	sender, _ := callbacker.NewSender(client, logger)

	retryCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	ok := sender.Send(server.URL, "test-token", callback)

	// Then
	assert.True(t, ok)
}
