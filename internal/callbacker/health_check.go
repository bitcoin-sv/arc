package callbacker

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/health/grpc_health_v1"
)

func (s *Server) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	err := s.callbacker.Health()
	if err != nil {
		s.logger.Error("callbacker unhealthy", slog.String("err", err.Error()))
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (s *Server) Watch(req *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	s.logger.Info("watching health", slog.String("service", req.Service))

	err := s.callbacker.Health()
	if err != nil {
		s.logger.Error("callbacker unhealthy", slog.String("err", err.Error()))
		return server.Send(&grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		})
	}

	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}
