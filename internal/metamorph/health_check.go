package metamorph

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

func (s *Server) List(ctx context.Context, _ *grpc_health_v1.HealthListRequest) (*grpc_health_v1.HealthListResponse, error) {
	mqStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if s.mq != nil {
		if s.mq.Status() == nats.CONNECTED {
			mqStatus = grpc_health_v1.HealthCheckResponse_SERVING
		}
	}

	storeStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if s.store.Ping(ctx) == nil {
		storeStatus = grpc_health_v1.HealthCheckResponse_SERVING
	}

	healthyConnections := 0
	for _, peer := range s.processor.GetPeers() {
		if peer.Connected() {
			healthyConnections++
			continue
		}
	}

	peerStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if healthyConnections > 0 {
		peerStatus = grpc_health_v1.HealthCheckResponse_SERVING
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
			"peers": {
				Status: peerStatus,
			},
		},
	}

	return listResp, nil
}

func (s *Server) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	s.logger.Debug("checking health", slog.String("service", req.Service))

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
	default:
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (s *Server) Watch(req *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	s.logger.Info("watching health", slog.String("service", req.Service))
	ctx := context.Background()
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
	default:
	}

	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}
