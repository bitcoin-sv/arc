package callbacker

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
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

func (s *Server) List(_ context.Context, _ *grpc_health_v1.HealthListRequest) (*grpc_health_v1.HealthListResponse, error) {
	mqStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if s.mqClient != nil {
		if s.mqClient.Status() == nats.CONNECTED {
			mqStatus = grpc_health_v1.HealthCheckResponse_SERVING
		}
	}

	storeStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if s.store.Ping() == nil {
		storeStatus = grpc_health_v1.HealthCheckResponse_SERVING
	}

	listResp := &grpc_health_v1.HealthListResponse{
		Statuses: map[string]*grpc_health_v1.HealthCheckResponse{
			"server": {
				Status: grpc_health_v1.HealthCheckResponse_SERVING,
			},
			"mq": {
				Status: mqStatus,
			},
			"store": {
				Status: storeStatus,
			},
		},
	}

	return listResp, nil
}

func (s *Server) Check(_ context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	switch req.Service {
	case readiness:
		err := s.store.Ping()
		if err != nil {
			s.logger.Error("no connection to DB", slog.String("err", err.Error()))
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}

		if s.mqClient == nil || !s.mqClient.IsConnected() {
			s.logger.Error("nats not connected")
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}

	case liveness:
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}, nil
	default:
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (s *Server) Watch(_ *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	err := s.store.Ping()
	if err != nil {
		s.logger.Error("no connection to DB", slog.String("err", err.Error()))
		return server.Send(&grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		})
	}

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
