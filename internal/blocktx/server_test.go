package blocktx

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
)

//go:generate moq -out ./peer_manager_mock.go . PeerManager

func TestStartGRPCServer(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "start and shutdown",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			storeMock := &store.BlocktxStoreMock{}
			pm := &PeerManagerMock{ShutdownFunc: func() {}}
			server := NewServer(storeMock, logger, pm, 0)

			err := server.StartGRPCServer("localhost:7000", 10000, "", logger)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond)

			server.Shutdown()
		})
	}
}

func TestDelUnfinishedBlock(t *testing.T) {
	tt := []struct {
		name                                  string
		getBlockHashesProcessingInProgressErr error
		delBlockProcessingErr                 error

		expectedRows     int64
		expectedErrorStr string
	}{
		{
			name: "success",

			expectedRows: 6,
		},
		{
			name:                                  "error - getBlockHashesProcessingInProgress",
			getBlockHashesProcessingInProgressErr: errors.New("failed to get block hashes processing in progress"),

			expectedErrorStr: "failed to get block hashes processing in progress",
			expectedRows:     0,
		},
		{
			name:                  "error - delBlockProcessingErr",
			delBlockProcessingErr: errors.New("failed to delete block processing error"),

			expectedErrorStr: "failed to delete block processing error",
			expectedRows:     0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			storeMock := &store.BlocktxStoreMock{
				GetBlockHashesProcessingInProgressFunc: func(ctx context.Context, processedBy string) ([]*chainhash.Hash, error) {
					return []*chainhash.Hash{testdata.TX1Hash, testdata.TX2Hash}, tc.getBlockHashesProcessingInProgressErr
				},

				DelBlockProcessingFunc: func(ctx context.Context, hash *chainhash.Hash, processedBy string) (int64, error) {
					return 3, tc.delBlockProcessingErr
				},
			}

			server := NewServer(storeMock, logger, nil, 0)

			resp, err := server.DelUnfinishedBlockProcessing(context.Background(), &blocktx_api.DelUnfinishedBlockProcessingRequest{
				ProcessedBy: "host",
			})

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.Equal(t, tc.expectedRows, resp.Rows)
		})
	}
}
