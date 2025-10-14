package blocktx_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
)

func TestCheck(t *testing.T) {
	tt := []struct {
		name           string
		service        string
		pingErr        error
		connectedPeers uint

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:    "liveness - serving",
			service: "liveness",
			pingErr: nil,

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:           "readiness - peer not found",
			service:        "readiness",
			pingErr:        nil,
			connectedPeers: 0,

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:    "readiness - db error - not connected",
			service: "readiness",
			pingErr: errors.New("not connected"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(_ context.Context, _ int) ([]*store.BlockGap, error) {
					return nil, nil
				},
				PingFunc: func(_ context.Context) error {
					return tc.pingErr
				},
			}

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			pm := &mocks.PeerManagerMock{
				CountConnectedPeersFunc: func() uint {
					return tc.connectedPeers
				},
			}
			serverCfg := grpc_utils.ServerConfig{}

			sut, err := blocktx.NewServer(logger, storeMock, pm, nil, serverCfg, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			// when
			resp, err := sut.Check(context.Background(), req)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, resp.Status)
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
			name:    "readiness - peer not found",
			service: "readiness",
			pingErr: nil,

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:    "readiness - db error - not connected",
			service: "readiness",
			pingErr: errors.New("not connected"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(_ context.Context, _ int) ([]*store.BlockGap, error) {
					return nil, nil
				},
				PingFunc: func(_ context.Context) error {
					return tc.pingErr
				},
			}

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			serverCfg := grpc_utils.ServerConfig{}
			pm := &mocks.PeerManagerMock{CountConnectedPeersFunc: func() uint { return 0 }}
			sut, err := blocktx.NewServer(logger, storeMock, pm, nil, serverCfg, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

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
		})
	}
}
