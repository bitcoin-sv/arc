package blocktx_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
	"github.com/nats-io/nats.go"
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
		testutils.RunParallel(t, false, tc.name, func(t *testing.T) {
			// given
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(_ context.Context, _ int) ([]*store.BlockGap, error) {
					return nil, nil
				},
				PingFunc: func() error {
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
		testutils.RunParallel(t, false, tc.name, func(t *testing.T) {
			// given
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(_ context.Context, _ int) ([]*store.BlockGap, error) {
					return nil, nil
				},
				PingFunc: func() error {
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

func TestList(t *testing.T) {
	tt := []struct {
		name           string
		pingErr        error
		connectedPeers uint

		expectedStatus *grpc_health_v1.HealthListResponse
	}{
		{
			name:           "success",
			pingErr:        nil,
			connectedPeers: 1,

			expectedStatus: &grpc_health_v1.HealthListResponse{
				Statuses: map[string]*grpc_health_v1.HealthCheckResponse{
					"server": {Status: grpc_health_v1.HealthCheckResponse_SERVING},
					"mq":     {Status: grpc_health_v1.HealthCheckResponse_SERVING},
					"store":  {Status: grpc_health_v1.HealthCheckResponse_SERVING},
					"peers":  {Status: grpc_health_v1.HealthCheckResponse_SERVING},
				},
			},
		},
		{
			name:           "readiness - peer not found",
			pingErr:        nil,
			connectedPeers: 0,

			expectedStatus: &grpc_health_v1.HealthListResponse{
				Statuses: map[string]*grpc_health_v1.HealthCheckResponse{
					"server": {Status: grpc_health_v1.HealthCheckResponse_SERVING},
					"mq":     {Status: grpc_health_v1.HealthCheckResponse_SERVING},
					"store":  {Status: grpc_health_v1.HealthCheckResponse_SERVING},
					"peers":  {Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING},
				},
			},
		},
		{
			name:           "readiness - db error - not connected",
			pingErr:        errors.New("not connected"),
			connectedPeers: 1,

			expectedStatus: &grpc_health_v1.HealthListResponse{
				Statuses: map[string]*grpc_health_v1.HealthCheckResponse{
					"server": {Status: grpc_health_v1.HealthCheckResponse_SERVING},
					"mq":     {Status: grpc_health_v1.HealthCheckResponse_SERVING},
					"store":  {Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING},
					"peers":  {Status: grpc_health_v1.HealthCheckResponse_SERVING},
				},
			},
		},
	}

	for _, tc := range tt {
		testutils.RunParallel(t, false, tc.name, func(t *testing.T) {
			// given
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(_ context.Context, _ int) ([]*store.BlockGap, error) {
					return nil, nil
				},
				PingFunc: func() error {
					return tc.pingErr
				},
			}
			mqClient := &mqMocks.MessageQueueClientMock{
				StatusFunc: func() nats.Status {
					return nats.CONNECTED
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			pm := &mocks.PeerManagerMock{
				CountConnectedPeersFunc: func() uint {
					return tc.connectedPeers
				},
			}
			serverCfg := grpc_utils.ServerConfig{}

			sut, err := blocktx.NewServer(logger, storeMock, pm, nil, serverCfg, 0, mqClient, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			// when
			resp, err := sut.List(context.Background(), &grpc_health_v1.HealthListRequest{})

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, resp)
		})
	}
}
