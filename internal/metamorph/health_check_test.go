package metamorph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestCheck(t *testing.T) {
	tt := []struct {
		name               string
		service            string
		pingErr            error
		processorHealthErr error
		statusNotSeen      int64

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:          "liveness - healthy",
			service:       "liveness",
			statusNotSeen: 0,

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:          "liveness - unhealthy",
			service:       "liveness",
			statusNotSeen: 1,

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
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
			metamorphStore := &storeMocks.MetamorphStoreMock{
				PingFunc: func(ctx context.Context) error {
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
				GetStatusNotSeenFunc: func() int64 { return tc.statusNotSeen },
			}

			server := metamorph.NewServer(metamorphStore, processor)

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
		statusNotSeen      int64

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:          "liveness - healthy",
			service:       "liveness",
			statusNotSeen: 0,

			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:          "liveness - unhealthy",
			service:       "liveness",
			statusNotSeen: 1,

			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
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
			metamorphStore := &storeMocks.MetamorphStoreMock{
				PingFunc: func(ctx context.Context) error {
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
				GetStatusNotSeenFunc: func() int64 { return tc.statusNotSeen },
			}

			server := metamorph.NewServer(metamorphStore, processor)

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
