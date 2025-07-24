package metamorph

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	readiness = "readiness"
	liveness  = "liveness"
)

type HealthWatchServer interface {
	Send(*grpc_health_v1.HealthCheckResponse) error
	grpc.ServerStream
}

func (s *Server) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	s.logger.Debug("checking health", slog.String("service", req.Service))
	//revive:disable:enforce-switch-style
	switch req.Service {
	case readiness:
		err := s.store.Ping(ctx)
		if err != nil {
			s.logger.Error("no connection to DB", slog.String("err", err.Error()))
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}

		err = s.processor.Health()
		if err != nil {
			s.logger.Error("processor unhealthy", slog.String("err", err.Error()))
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}
	case liveness:
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}, nil
	}
	//revive:enable:enforce-switch-style

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (s *Server) Watch(req *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	s.logger.Info("watching health", slog.String("service", req.Service))
	ctx := context.Background()
	//revive:disable:enforce-switch-style
	switch req.Service {
	case readiness:
		err := s.store.Ping(ctx)
		if err != nil {
			s.logger.Error("no connection to DB", slog.String("err", err.Error()))
			return server.Send(&grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			})
		}

		err = s.processor.Health()
		if err != nil {
			s.logger.Error("processor unhealthy", slog.String("err", err.Error()))
			return server.Send(&grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			})
		}

	case liveness:
		return server.Send(&grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		})
	}
	//revive:enable:enforce-switch-style

	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}
