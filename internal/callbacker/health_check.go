package callbacker

import (
	"context"
	"google.golang.org/grpc"

	"google.golang.org/grpc/health/grpc_health_v1"
)

type HealthWatchServer interface {
	Send(*grpc_health_v1.HealthCheckResponse) error
	grpc.ServerStream
}

func (s *Server) Check(_ context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if s.mqClient == nil || !s.mqClient.IsConnected() {
		s.logger.Error("nats not connected")
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (s *Server) Watch(_ *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	if s.mqClient == nil || !s.mqClient.IsConnected() {
		s.logger.Error("nats not connected")
		return server.Send(&grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		})
	}

	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}
