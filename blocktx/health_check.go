package blocktx

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	readiness = "readiness"
)

//go:generate moq -pkg mocks -out ./mocks/health_watch_server_mock.go . HealthWatchServer
type HealthWatchServer interface {
	Send(*grpc_health_v1.HealthCheckResponse) error
	grpc.ServerStream
}

func (s *Server) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {

	s.logger.Debug("checking health", slog.String("service", req.Service))

	if req.Service == readiness {
		// check db connection
		_, err := s.store.GetPrimary(ctx)
		if err != nil {
			s.logger.Error("no connection to DB", slog.String("err", err.Error()))
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}

		// verify we have at least 2 nodes connected to blocktx
		healthy := false
		for _, peer := range s.ph.peers {
			if peer.IsHealthy() {
				healthy = true
				break
			}
		}

		if !healthy {
			s.logger.Error("healthy peer not found")
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}
