package merkle_verifier

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

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
