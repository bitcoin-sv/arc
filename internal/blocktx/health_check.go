package blocktx

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	readiness = "readiness"
)

type HealthWatchServer interface {
	Send(*grpc_health_v1.HealthCheckResponse) error
	grpc.ServerStream
}

func (s *Server) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	s.logger.Debug("checking health", slog.String("service", req.Service))

	if req.Service == readiness {
		// check db connection
		err := s.store.Ping(ctx)
		if err != nil {
			s.logger.Error("no connection to DB", slog.String("err", err.Error()))
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}

		// verify we have at least 1 node connected to blocktx
		healthy := s.pm.CountConnectedPeers() > 0

		if !healthy {
			s.logger.Error("healthy peer not found")
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}

		if s.processor.mqClient == nil || s.processor.mqClient.Status() != nats.CONNECTED {
			s.logger.Error("nats not connected")
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
		// check db connection
		err := s.store.Ping(ctx)
		if err != nil {
			s.logger.Error("no connection to DB", slog.String("err", err.Error()))
			return server.Send(&grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			})
		}

		// verify we have at least 1 node connected to blocktx
		healthy := s.pm.CountConnectedPeers() > 0

		if !healthy {
			s.logger.Error("healthy peer not found")
			return server.Send(&grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			})
		}
	}

	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}
