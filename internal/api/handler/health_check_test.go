package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/bitcoin-sv/arc/internal/api/mocks"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
)

func TestCheck(t *testing.T) {
	tt := []struct {
		name         string
		service      string
		currentBlock int32

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:         "readiness - serving",
			service:      "readiness",
			currentBlock: int32(1),

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:         "readiness - block not set",
			service:      "readiness",
			currentBlock: int32(0),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			serverCfg := grpc_utils.ServerConfig{}

			mockedArcDefaultHandlerHealth := &mocks.ArcDefaultHandlerHealthMock{
				CurrentBlockHeightFunc: func() int32 {
					return tc.currentBlock
				},
			}
			sut, err := NewServer(testLogger, mockedArcDefaultHandlerHealth, serverCfg)
			require.NoError(t, err)
			defer sut.Shutdown()

			// when
			resp, err := sut.Check(context.Background(), req)
			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, resp.Status)
		})
	}
}

func TestList(t *testing.T) {
	tt := []struct {
		name         string
		service      string
		currentBlock int32

		expectedStatus *grpc_health_v1.HealthListResponse
	}{
		{
			name:         "readiness - serving",
			service:      "readiness",
			currentBlock: int32(1),

			expectedStatus: &grpc_health_v1.HealthListResponse{
				Statuses: map[string]*grpc_health_v1.HealthCheckResponse{
					"server": {Status: grpc_health_v1.HealthCheckResponse_SERVING},
				},
			},
		},
		{
			name:         "readiness - block not set",
			service:      "readiness",
			currentBlock: int32(0),

			expectedStatus: &grpc_health_v1.HealthListResponse{
				Statuses: map[string]*grpc_health_v1.HealthCheckResponse{
					"server": {Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			serverCfg := grpc_utils.ServerConfig{}

			mockedArcDefaultHandlerHealth := &mocks.ArcDefaultHandlerHealthMock{
				CurrentBlockHeightFunc: func() int32 {
					return tc.currentBlock
				},
			}
			sut, err := NewServer(testLogger, mockedArcDefaultHandlerHealth, serverCfg)
			require.NoError(t, err)
			defer sut.Shutdown()

			// when
			resp, err := sut.List(context.Background(), &grpc_health_v1.HealthListRequest{})
			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, resp)
		})
	}
}
