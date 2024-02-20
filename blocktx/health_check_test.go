package blocktx

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/metamorph/mocks"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestCheck(t *testing.T) {
	tt := []struct {
		name               string
		service            string
		pingErr            error
		processorHealthErr error
		getBlockErr        error

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:        "liveness - healthy",
			service:     "liveness",
			getBlockErr: errors.New("failed to get block"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:        "readiness - healthy",
			service:     "readiness",
			getBlockErr: errors.New("failed to get block"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:        "readiness - ping error",
			service:     "readiness",
			pingErr:     errors.New("no connection"),
			getBlockErr: errors.New("failed to get block"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:               "readiness - unhealthy processor",
			service:            "readiness",
			processorHealthErr: errors.New("unhealthy processor"),
			getBlockErr:        errors.New("failed to get block"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			storeMock := &store.InterfaceMock{
				GetBlockFunc: func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
					return &blocktx_api.Block{}, tc.getBlockErr
				},
				InsertBlockFunc: func(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
					return 0, nil
				},
				MarkBlockAsDoneFunc: func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
					return nil
				},
				GetPrimaryFunc: func(ctx context.Context) (string, error) {
					hostName, err := os.Hostname()
					return hostName, err
				},
				TryToBecomePrimaryFunc: func(ctx context.Context, myHostName string) error {
					return nil
				},
			}

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			// build peer manager
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			txChan := make(chan []byte, 10)

			txChan <- testdata.TX1Hash[:]
			txChan <- testdata.TX2Hash[:]
			txChan <- testdata.TX3Hash[:]
			txChan <- testdata.TX4Hash[:]

			peerHandler, _ := NewPeerHandler(
				logger,
				storeMock,
				100,
				[]string{},
				wire.TestNet,
				WithRegisterTxsInterval(time.Millisecond*20),
				WithTxChan(txChan),
				WithRegisterTxsBatchSize(3),
			)

			server := NewServer(storeMock, logger, peerHandler)

			resp, err := server.Check(context.Background(), req)
			require.NoError(t, err)

			require.Equal(t, tc.expectedStatus, resp.Status)
		})
	}
}

func TestWatch(t *testing.T) {
	tt := []struct {
		name               string
		service            string
		pingErr            error
		processorHealthErr error
		getBlockErr        error

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:               "liveness - healthy",
			service:            "liveness",
			processorHealthErr: errors.New("unhealthy processor"),
			getBlockErr:        errors.New("failed to get block"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:               "readiness - healty",
			service:            "readiness",
			processorHealthErr: errors.New("unhealthy processor"),
			getBlockErr:        errors.New("failed to get block"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:               "readiness - ping error",
			service:            "readiness",
			pingErr:            errors.New("no connection"),
			processorHealthErr: errors.New("unhealthy processor"),
			getBlockErr:        errors.New("failed to get block"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:               "readiness - unhealthy processor",
			service:            "readiness",
			processorHealthErr: errors.New("unhealthy processor"),
			getBlockErr:        errors.New("failed to get block"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			storeMock := &store.InterfaceMock{
				GetBlockFunc: func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
					return &blocktx_api.Block{}, tc.getBlockErr
				},
				InsertBlockFunc: func(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
					return 0, nil
				},
				MarkBlockAsDoneFunc: func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
					return nil
				},
				GetPrimaryFunc: func(ctx context.Context) (string, error) {
					hostName, err := os.Hostname()
					return hostName, err
				},
				TryToBecomePrimaryFunc: func(ctx context.Context, myHostName string) error {
					return nil
				},
			}

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			// build peer manager
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			txChan := make(chan []byte, 10)

			txChan <- testdata.TX1Hash[:]
			txChan <- testdata.TX2Hash[:]
			txChan <- testdata.TX3Hash[:]
			txChan <- testdata.TX4Hash[:]

			peerHandler, _ := NewPeerHandler(
				logger,
				storeMock,
				100,
				[]string{},
				wire.TestNet,
				WithRegisterTxsInterval(time.Millisecond*20),
				WithTxChan(txChan),
				WithRegisterTxsBatchSize(3),
			)

			server := NewServer(storeMock, logger, peerHandler)

			watchServer := &mocks.HealthWatchServerMock{
				SendFunc: func(healthCheckResponse *grpc_health_v1.HealthCheckResponse) error {
					require.Equal(t, tc.expectedStatus, healthCheckResponse.Status)
					return nil
				},
			}

			err := server.Watch(req, watchServer)
			require.NoError(t, err)
		})
	}
}
