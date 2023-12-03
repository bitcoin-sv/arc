package jobs

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/bitcoin-sv/arc/background_worker/jobs/mock"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

//go:generate moq -pkg mock -out ./mock/blocktx_api_client_mock.go ../../blocktx/blocktx_api BlockTxAPIClient

func TestClearBlocktxTransactions(t *testing.T) {
	tt := []struct {
		name     string
		response *blocktx_api.ClearDataResponse
		clearErr error

		expectedErrorStr string
	}{
		{
			name: "success",
			response: &blocktx_api.ClearDataResponse{
				Rows: 100,
			},
		},
		{
			name:     "error",
			clearErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			client := &mock.BlockTxAPIClientMock{
				ClearTransactionsFunc: func(ctx context.Context, in *blocktx_api.ClearData, opts ...grpc.CallOption) (*blocktx_api.ClearDataResponse, error) {
					return tc.response, tc.clearErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			blocktxJob := NewBlocktx(client, 14, logger)

			err := blocktxJob.ClearTransactions()
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClearBlocks(t *testing.T) {
	tt := []struct {
		name     string
		response *blocktx_api.ClearDataResponse
		clearErr error

		expectedErrorStr string
	}{
		{
			name: "success",
			response: &blocktx_api.ClearDataResponse{
				Rows: 100,
			},
		},
		{
			name:     "error",
			clearErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			client := &mock.BlockTxAPIClientMock{
				ClearBlocksFunc: func(ctx context.Context, in *blocktx_api.ClearData, opts ...grpc.CallOption) (*blocktx_api.ClearDataResponse, error) {
					return tc.response, tc.clearErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			blocktxJob := NewBlocktx(client, 14, logger)

			err := blocktxJob.ClearBlocks()
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClearBlockTransactionsMap(t *testing.T) {
	tt := []struct {
		name     string
		response *blocktx_api.ClearDataResponse
		clearErr error

		expectedErrorStr string
	}{
		{
			name: "success",
			response: &blocktx_api.ClearDataResponse{
				Rows: 100,
			},
		},
		{
			name:     "error",
			clearErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			client := &mock.BlockTxAPIClientMock{
				ClearBlockTransactionsMapFunc: func(ctx context.Context, in *blocktx_api.ClearData, opts ...grpc.CallOption) (*blocktx_api.ClearDataResponse, error) {
					return tc.response, tc.clearErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			blocktxJob := NewBlocktx(client, 14, logger)

			err := blocktxJob.ClearBlockTransactionsMap()
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}
		})
	}
}
