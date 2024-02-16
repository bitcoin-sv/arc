package jobs

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/bitcoin-sv/arc/background_worker/jobs/mock"
	"github.com/stretchr/testify/require"
)

//go:generate moq -pkg mock -out ./mock/blocktx_client_mock.go ../../blocktx ClientI

func TestClearBlocktxTransactions(t *testing.T) {
	tt := []struct {
		name     string
		response int64
		clearErr error

		expectedErrorStr string
	}{
		{
			name:     "success",
			response: 100,
		},
		{
			name:     "error",
			clearErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			client := &mock.ClientIMock{
				ClearTransactionsFunc: func(ctx context.Context, retentionDays int32) (int64, error) {
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
		response int64
		clearErr error

		expectedErrorStr string
	}{
		{
			name:     "success",
			response: 100,
		},
		{
			name:     "error",
			clearErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			client := &mock.ClientIMock{
				ClearBlocksFunc: func(ctx context.Context, retentionDays int32) (int64, error) {
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
		response int64
		clearErr error

		expectedErrorStr string
	}{
		{
			name:     "success",
			response: 100,
		},
		{
			name:     "error",
			clearErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			client := &mock.ClientIMock{
				ClearBlockTransactionsMapFunc: func(ctx context.Context, retentionDays int32) (int64, error) {
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
