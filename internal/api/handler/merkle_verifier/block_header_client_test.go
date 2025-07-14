package merkle_verifier

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	bhsDomains "github.com/bitcoin-sv/block-headers-service/domains"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/validator/beef"
)

func TestClient_checkChainTrackers(t *testing.T) {
	tt := []struct {
		name       string
		httpStatus int
		timeout    bool

		expectedAvailability bool
	}{
		{
			name:       "block header service available",
			httpStatus: http.StatusOK,

			expectedAvailability: true,
		},
		{
			name:       "block header service not available",
			httpStatus: http.StatusServiceUnavailable,

			expectedAvailability: false,
		},
		{
			name:       "block header service timing out",
			httpStatus: http.StatusOK,
			timeout:    true,

			expectedAvailability: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer abc", r.Header.Get("Authorization"))
				if tc.timeout {
					time.Sleep(1 * time.Second)
				}
				w.WriteHeader(tc.httpStatus)
			}))
			defer server.Close()

			ct := NewChainTracker(server.URL, "abc")

			chainTrackers := []*ChainTracker{ct}

			sut := NewClient(logger, chainTrackers, WithCheckChainTrackersInterval(100*time.Millisecond), WithTimeout(300*time.Second))
			defer sut.Shutdown()

			time.Sleep(200 * time.Millisecond)

			checkChainTrackers(context.TODO(), sut)

			require.Equal(t, tc.expectedAvailability, ct.IsAvailable())
		})
	}
}

func TestClient_IsValidRootForHeight(t *testing.T) {
	tt := []struct {
		name          string
		httpStatus    int
		timeout       bool
		responseBody  any
		isUnavailable bool

		expectedError error
		expectedOk    bool
	}{
		{
			name:       "root is valid",
			httpStatus: http.StatusOK,
			responseBody: IsValidRootForHeightResponse{
				ConfirmationState: bhsDomains.Confirmed,
			},

			expectedOk: true,
		},
		{
			name:       "http status unavailable",
			httpStatus: http.StatusServiceUnavailable,

			expectedError: beef.ErrRequestFailed,
			expectedOk:    false,
		},
		{
			name:       "root is invalid",
			httpStatus: http.StatusOK,
			responseBody: IsValidRootForHeightResponse{
				ConfirmationState: bhsDomains.Invalid,
			},

			expectedOk: false,
		},
		{
			name:       "unable to verify root",
			httpStatus: http.StatusOK,
			responseBody: IsValidRootForHeightResponse{
				ConfirmationState: bhsDomains.UnableToVerify,
			},

			expectedOk: false,
		},
		{
			name:       "time out",
			httpStatus: http.StatusOK,
			responseBody: IsValidRootForHeightResponse{
				ConfirmationState: bhsDomains.Confirmed,
			},
			timeout: true,

			expectedOk:    false,
			expectedError: beef.ErrRequestTimedOut,
		},
		{
			name:         "failed to parse response",
			httpStatus:   http.StatusOK,
			responseBody: []string{},

			expectedOk:    false,
			expectedError: ErrParseResponse,
		},
		{
			name:          "all unavailable",
			httpStatus:    http.StatusOK,
			isUnavailable: true,

			expectedOk:    false,
			expectedError: beef.ErrNoChainTrackersAvailable,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.String(), "status") {
					if tc.isUnavailable {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.WriteHeader(http.StatusOK)
					return
				}

				require.Equal(t, "Bearer abc", r.Header.Get("Authorization"))
				if tc.timeout {
					time.Sleep(300 * time.Millisecond)
					t.Log("timeout")
				}
				w.WriteHeader(tc.httpStatus)

				jsonResp, err := json.Marshal(tc.responseBody)
				require.NoError(t, err)
				_, err = w.Write(jsonResp)
				require.NoError(t, err)
			}))
			defer server.Close()

			ct := NewChainTracker(server.URL, "abc")

			chainTrackers := []*ChainTracker{ct}

			sut := NewClient(logger, chainTrackers, WithCheckChainTrackersInterval(100*time.Millisecond), WithTimeout(100*time.Millisecond))
			defer sut.Shutdown()

			root, err := chainhash.NewHashFromHex("7382df1b717287ab87e5e3e25759697c4c45eea428f701cdd0c77ad3fc707257")
			require.NoError(t, err)

			time.Sleep(200 * time.Millisecond)

			ok, err := sut.IsValidRootForHeight(context.TODO(), root, 800000)
			require.Equal(t, !tc.isUnavailable, ct.IsAvailable())
			require.Equal(t, tc.expectedOk, ok)

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestClient_CurrentHeight(t *testing.T) {
	tt := []struct {
		name          string
		httpStatus    int
		timeout       bool
		responseBody  any
		isUnavailable bool

		expectedError  error
		expectedHeight uint32
	}{
		{
			name:       "root is valid",
			httpStatus: http.StatusOK,
			responseBody: State{
				Height: 10000,
			},

			expectedHeight: 10000,
		},
		{
			name:       "http status unavailable",
			httpStatus: http.StatusServiceUnavailable,

			expectedError:  beef.ErrRequestFailed,
			expectedHeight: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.String(), "status") {
					if tc.isUnavailable {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.WriteHeader(http.StatusOK)
					return
				}

				require.Equal(t, "Bearer abc", r.Header.Get("Authorization"))
				if tc.timeout {
					time.Sleep(300 * time.Millisecond)
					t.Log("timeout")
				}
				w.WriteHeader(tc.httpStatus)

				jsonResp, err := json.Marshal(tc.responseBody)
				require.NoError(t, err)
				_, err = w.Write(jsonResp)
				require.NoError(t, err)
			}))
			defer server.Close()

			ct := NewChainTracker(server.URL, "abc")

			chainTrackers := []*ChainTracker{ct}

			sut := NewClient(logger, chainTrackers, WithCheckChainTrackersInterval(100*time.Millisecond), WithTimeout(100*time.Millisecond))
			defer sut.Shutdown()

			time.Sleep(200 * time.Millisecond)

			height, err := sut.CurrentHeight(context.TODO())
			require.Equal(t, !tc.isUnavailable, ct.IsAvailable())
			require.Equal(t, tc.expectedHeight, height)

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}
