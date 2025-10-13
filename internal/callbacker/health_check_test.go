package callbacker_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/mq"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
)

func TestCheck(t *testing.T) {
	mqClient := &mqMocks.MessageQueueClientMock{
		StatusFunc: func() nats.Status {
			return nats.CONNECTED
		},
		IsConnectedFunc: func() bool {
			return true
		},
	}

	tt := []struct {
		name               string
		service            string
		mqClient           mq.MessageQueueClient
		pingErr            error
		processorHealthErr error
		setURLMappingErr   error
		mappings           map[string]string

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:           "liveness - unhealthy",
			service:        "liveness",
			mqClient:       nil,
			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:           "readiness - healthy",
			service:        "readiness",
			mqClient:       mqClient,
			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			sut, err := callbacker.NewServer(slog.Default(), nil, tc.mqClient, grpc_utils.ServerConfig{})
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
	mqClient := &mqMocks.MessageQueueClientMock{
		StatusFunc: func() nats.Status {
			return nats.CONNECTED
		},
		IsConnectedFunc: func() bool {
			return true
		},
	}
	tt := []struct {
		name               string
		service            string
		mqClient           mq.MessageQueueClient
		pingErr            error
		processorHealthErr error
		setURLMappingErr   error
		mappings           map[string]string

		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:           "liveness - healthy",
			service:        "liveness",
			mqClient:       mqClient,
			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:           "readiness - not healthy",
			service:        "readiness",
			mqClient:       nil,
			expectedStatus: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tc.service,
			}

			sut, err := callbacker.NewServer(slog.Default(), nil, tc.mqClient, grpc_utils.ServerConfig{})
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

// Todo: test List
