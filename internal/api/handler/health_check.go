package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	readiness = "readiness"
)

type HealthWatchServer interface {
	grpc.ServerStream
}

func (s *Server) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	s.logger.Debug("checking health", slog.String("service", req.Service))
	if req.Service == readiness {
		if s.handler.CurrentBlockHeight() == 0 {
			s.logger.Error("current block not set yet")
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (s *Server) Watch(req *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	s.logger.Info("watching health", slog.String("service", req.Service))
	if req.Service == readiness {
		if s.handler.CurrentBlockHeight() == 0 {
			s.logger.Error("current block not set yet")
			return server.Send(&grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			})
		}
	}

	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}
