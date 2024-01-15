package metamorph

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

	s.logger.Info("checking health", slog.String("service", req.Service))

	if req.Service == readiness {
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

		err = s.btc.Health(ctx)
		if err != nil {
			s.logger.Error("btc connection unhealthy", slog.String("err", err.Error()))
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
	ctx := context.Background()
	if req.Service == readiness {
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

		err = s.btc.Health(ctx)
		if err != nil {
			s.logger.Error("btc connection unhealthy", slog.String("err", err.Error()))
			return server.Send(&grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			})
		}
	}

	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}
