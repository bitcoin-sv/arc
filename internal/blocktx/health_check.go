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

func (s *Server) List(ctx context.Context, _ *grpc_health_v1.HealthListRequest) (*grpc_health_v1.HealthListResponse, error) {
	mqStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if s.mqClient != nil {
		if s.mqClient.Status() == nats.CONNECTED {
			mqStatus = grpc_health_v1.HealthCheckResponse_SERVING
		}
	}

	storeStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if s.store.Ping(ctx) == nil {
		storeStatus = grpc_health_v1.HealthCheckResponse_SERVING
	}

	peerStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if s.pm.CountConnectedPeers() > 0 {
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
