package metamorph_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
)

func TestCheck(t *testing.T) {
	tt := []struct {
		name               string
		service            string
		pingErr            error
		processorHealthErr error

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:    "liveness - healthy",
			service: "liveness",

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:    "readiness - healthy",
			service: "readiness",

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:    "readiness - ping error",
			service: "readiness",
			pingErr: errors.New("no connection"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:               "readiness - unhealthy processor",
			service:            "readiness",
			processorHealthErr: errors.New("unhealthy processor"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{
				PingFunc: func(_ context.Context) error {
					return tc.pingErr
				},
			}

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			processor := &mocks.ProcessorIMock{
				HealthFunc: func() error {
					return tc.processorHealthErr
				},
			}

			sut, err := metamorph.NewServer(slog.Default(), metamorphStore, processor, grpc_utils.ServerConfig{})
			require.NoError(t, err)
			defer sut.GracefulStop()

			// when
			resp, err := sut.Check(context.Background(), req)
			require.NoError(t, err)

			// then
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

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:    "liveness - healthy",
			service: "liveness",

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:    "readiness - healty",
			service: "readiness",

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:    "readiness - ping error",
			service: "readiness",
			pingErr: errors.New("no connection"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:               "readiness - unhealthy processor",
			service:            "readiness",
			processorHealthErr: errors.New("unhealthy processor"),

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			metamorphStore := &storeMocks.MetamorphStoreMock{
				PingFunc: func(_ context.Context) error {
					return tc.pingErr
				},
			}

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			processor := &mocks.ProcessorIMock{
				HealthFunc: func() error {
					return tc.processorHealthErr
				},
			}

			sut, err := metamorph.NewServer(slog.Default(), metamorphStore, processor, grpc_utils.ServerConfig{})
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
