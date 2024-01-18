package jobs

import (
	"context"
	"errors"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"log/slog"
	"os"
	"testing"

	"github.com/bitcoin-sv/arc/background_worker/jobs/mock"
)

//go:generate moq -pkg mock -out ./mock/metamorph_api_client_mock.go ../../metamorph/metamorph_api MetaMorphAPIClient
func TestClearTransactions(t *testing.T) {
	tt := []struct {
		name     string
		clearErr error

		expectedErrorStr string
	}{
		{
			name: "success",
		},
		{
			name:     "error",
			clearErr: errors.New("failed to clear"),

			expectedErrorStr: "failed to clear",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			client := &mock.MetaMorphAPIClientMock{
				ClearDataFunc: func(ctx context.Context, in *metamorph_api.ClearDataRequest, opts ...grpc.CallOption) (*metamorph_api.ClearDataResponse, error) {
					return &metamorph_api.ClearDataResponse{}, tc.clearErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			job := NewMetamorph(client, 10, logger)
			err := job.ClearTransactions()

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}
		})
	}
}
