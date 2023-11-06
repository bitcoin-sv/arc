package blocktx

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

//go:generate moq -pkg mock -out ./blocktx_api/mock/blocktx_api_client_mock.go ./blocktx_api BlockTxAPIClient
//go:generate moq -pkg mock -out ./blocktx_api/mock/blocktx_notification_client_mock.go ./blocktx_api BlockTxAPI_GetBlockNotificationStreamClient
func TestStart(t *testing.T) {

	blockHash, err := hex.DecodeString("0000000000000497b672795cad5e839f63fc43d00ce675836e55338faecc3871")
	require.NoError(t, err)

	tt := []struct {
		name         string
		getStreamErr error
		recvErr      error
	}{
		{
			name: "success",
		},
		{
			name:         "get stream error",
			getStreamErr: errors.New("failed to get stream"),
		},
		{
			name:    "recv error",
			recvErr: errors.New("failed to receive item"),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			blockTx := &mock.BlockTxAPIClientMock{
				GetBlockNotificationStreamFunc: func(ctx context.Context, in *blocktx_api.Height, opts ...grpc.CallOption) (blocktx_api.BlockTxAPI_GetBlockNotificationStreamClient, error) {
					return &mock.BlockTxAPI_GetBlockNotificationStreamClientMock{
						RecvFunc: func() (*blocktx_api.Block, error) {
							time.Sleep(time.Millisecond * 2)
							return &blocktx_api.Block{
								Hash: blockHash,
							}, tc.recvErr

						},
					}, tc.getStreamErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			client := NewClient(blockTx,
				WithRetryInterval(time.Millisecond*10),
				WithLogger(logger),
			)

			client.Start(make(chan *blocktx_api.Block, 10))
			time.Sleep(time.Millisecond * 15)
			client.Shutdown()

			require.NoError(t, err)
		})
	}
}
