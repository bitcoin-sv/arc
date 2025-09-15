package callbacker_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker"
)

func TestCallbackSender_Send(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		useWrongURL    bool

		expectedSuccess bool
		expectedRetries int
		expectedRetry   bool
	}{
		{
			name:           "success - callback sent",
			responseStatus: http.StatusOK,

			expectedSuccess: true,
			expectedRetry:   false,
			expectedRetries: 1,
		},
		{
			name:           "retry - server error and fails after retries",
			responseStatus: http.StatusInternalServerError,

			expectedSuccess: false,
			expectedRetry:   true,
			expectedRetries: 5, // Adjust based on your retry logic in `Send`
		},
		{
			name:        "retry - server error and fails after retries",
			useWrongURL: true,

			expectedSuccess: false,
			expectedRetry:   false,
			expectedRetries: 0, // Adjust based on your retry logic in `Send`
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			retries := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.responseStatus)
				retries++
				time.Sleep(1 * time.Millisecond)
			}))
			defer server.Close()

			logger := slog.Default()
			sut, err := callbacker.NewSender(logger, callbacker.WithTimeout(500*time.Second))
			require.NoError(t, err)

			defer sut.GracefulStop()

			url := server.URL
			if tc.useWrongURL {
				url = "https://abc.not-existing.abc"
			}

			//When
			success, retry := sut.Send(url, "test-token", &callbacker.Callback{TxID: "1234", TxStatus: "SEEN_ON_NETWORK"})
			//Then
			require.Equal(t, tc.expectedSuccess, success, "Expected success to be %v, but got %v", tc.expectedSuccess, success)
			require.Equal(t, tc.expectedRetry, retry, "Expected retry to be %v, but got %v", tc.expectedRetry, retry)
			require.Equal(t, tc.expectedRetries, retries, "Expected retries to be %d, but got %d", tc.expectedRetries, retries)
		})
	}
}

func TestCallbackSender_GracefulStop(t *testing.T) {
	// Given
	logger := slog.Default()
	sut, err := callbacker.NewSender(logger)
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
	logger := slog.Default()
	sut, err := callbacker.NewSender(logger)
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
		name           string
		url            string
		token          string
		dtos           []*callbacker.Callback
		responseStatus int

		expectedSuccess bool
		expectedRetries int
	}{
		{
			name:           "success - batch sent with retries",
			url:            "/batch",
			token:          "test-token",
			dtos:           []*callbacker.Callback{{TxID: "1234", TxStatus: "SEEN_ON_NETWORK"}, {TxID: "5678", TxStatus: "MINED"}},
			responseStatus: http.StatusOK,

			expectedSuccess: true,
			expectedRetries: 1,
		},
		{
			name:           "failure - server error on batch with retries",
			url:            "/batch",
			token:          "test-token",
			dtos:           []*callbacker.Callback{{TxID: "1234", TxStatus: "SEEN_ON_NETWORK"}, {TxID: "5678", TxStatus: "MINED"}},
			responseStatus: http.StatusInternalServerError,

			expectedSuccess: false,
			expectedRetries: 5,
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

				w.WriteHeader(tc.responseStatus)

				retries++
				time.Sleep(5 * time.Millisecond)
			}))
			defer server.Close()

			logger := slog.Default()
			sut, err := callbacker.NewSender(logger, callbacker.WithTimeout(2*time.Second))
			require.NoError(t, err)
			defer sut.GracefulStop()

			// When
			success, _ := sut.SendBatch(server.URL+tc.url, tc.token, tc.dtos)

			// Then
			require.Equal(t, tc.expectedSuccess, success, "Expected success to be %v, but got %v", tc.expectedSuccess, success)
			require.Equal(t, tc.expectedRetries, retries, "Expected retries to be %d, but got %d", tc.expectedRetries, retries)
		})
	}
}
