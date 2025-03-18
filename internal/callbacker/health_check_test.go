package callbacker_test

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/mq"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/nats-io/nats.go"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
)

func TestCheck(t *testing.T) {
	mqClient := &mqMocks.MessageQueueClientMock{
		StatusFunc: func() string {
			return nats.CONNECTED.String()
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
			dispatcher := &mocks.DispatcherMock{DispatchFunc: func(url string, dto *callbacker.CallbackEntry) {
				switch url {
				case "https://example.com/callback1":
					require.Equal(t, dto.Token, "token1")
				case "https://example.com/callback2":
					require.Equal(t, dto.Token, "token2")
				default:
					t.Fatalf("unexpected callback URL: %s", url)
				}
			}}

			sut, err := callbacker.NewServer(slog.Default(), dispatcher, nil, tc.mqClient, grpc_utils.ServerConfig{})
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
		StatusFunc: func() string {
			return nats.CONNECTED.String()
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

			dispatcher := &mocks.DispatcherMock{DispatchFunc: func(url string, dto *callbacker.CallbackEntry) {
				switch url {
				case "https://example.com/callback1":
					require.Equal(t, dto.Token, "token1")
				case "https://example.com/callback2":
					require.Equal(t, dto.Token, "token2")
				default:
					t.Fatalf("unexpected callback URL: %s", url)
				}
			}}
			sut, err := callbacker.NewServer(slog.Default(), dispatcher, nil, tc.mqClient, grpc_utils.ServerConfig{})
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
