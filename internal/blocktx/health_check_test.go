package blocktx_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/libsv/go-p2p"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestCheck(t *testing.T) {
	tt := []struct {
		name    string
		service string
		pingErr error

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:    "liveness - peer not found",
			service: "readiness",
			pingErr: nil,

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:    "db error - not connected",
			service: "readiness",
			pingErr: errors.New("not connected"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:    "db error - not connected",
			service: "readiness",
			pingErr: nil,

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			const batchSize = 4

			// given
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(ctx context.Context, heightRange int) ([]*store.BlockGap, error) {
					return nil, nil
				},
				PingFunc: func(ctx context.Context) error {
					return tc.pingErr
				},
			}

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			processor, err := blocktx.NewProcessor(logger, storeMock, nil, nil, blocktx.WithTransactionBatchSize(batchSize))
			require.NoError(t, err)
			pm := &mocks.PeerManagerMock{GetPeersFunc: func() []p2p.PeerI {
				return []p2p.PeerI{&mocks.PeerMock{
					IsHealthyFunc: func() bool {
						return false
					},
					ConnectedFunc: func() bool {
						return false
					},
				}}
			}}
			sut := blocktx.NewServer(storeMock, logger, pm, 0)

			// when
			resp, err := sut.Check(context.Background(), req)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, resp.Status)

			// cleanup
			processor.Shutdown()
		})

	}
}

func TestWatch(t *testing.T) {
	tt := []struct {
		name    string
		service string
		pingErr error

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:    "liveness - healthy",
			service: "liveness",
			pingErr: nil,

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:    "not ready - healthy",
			service: "readiness",
			pingErr: nil,

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:    "not ready - healthy",
			service: "readiness",
			pingErr: errors.New("not connected"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			const batchSize = 4

			// given
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(ctx context.Context, heightRange int) ([]*store.BlockGap, error) {
					return nil, nil
				},
				PingFunc: func(ctx context.Context) error {
					return tc.pingErr
				},
			}

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			processor, err := blocktx.NewProcessor(logger, storeMock, nil, nil, blocktx.WithTransactionBatchSize(batchSize))
			require.NoError(t, err)

			pm := &mocks.PeerManagerMock{
				GetPeersFunc: func() []p2p.PeerI {
					return []p2p.PeerI{
						&mocks.PeerMock{
							IsHealthyFunc: func() bool { return false },
							ConnectedFunc: func() bool { return false },
						},
					}
				},
			}

			sut := blocktx.NewServer(storeMock, logger, pm, 0)

			watchServer := &mocks.HealthWatchServerMock{
				SendFunc: func(healthCheckResponse *grpc_health_v1.HealthCheckResponse) error {
					require.Equal(t, tc.expectedStatus, healthCheckResponse.Status)
					return nil
				},
			}

			// when
			err = sut.Watch(req, watchServer)

			// then
			require.NoError(t, err)

			// cleanup
			processor.Shutdown()
		})

	}
}
