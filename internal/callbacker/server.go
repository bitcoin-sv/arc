package callbacker

import (
	"context"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/mq"
)

type Server struct {
	callbacker_api.UnimplementedCallbackerAPIServer
	grpc_utils.GrpcServer
	store    store.ProcessorStore
	mqClient mq.MessageQueueClient
	logger   *slog.Logger
}

// NewServer will return a server instance
func NewServer(logger *slog.Logger, callbackerStore store.ProcessorStore, mqClient mq.MessageQueueClient, cfg grpc_utils.ServerConfig) (*Server, error) {
	grpcServer, err := grpc_utils.NewGrpcServer(logger, cfg)
	if err != nil {
		return nil, err
	}

	s := &Server{
		GrpcServer: grpcServer,
		store:      callbackerStore,
		logger:     logger,
		mqClient:   mqClient,
	}
	// register health server endpoint
	grpc_health_v1.RegisterHealthServer(grpcServer.Srv, s)

	callbacker_api.RegisterCallbackerAPIServer(s.GrpcServer.Srv, s)
	reflection.Register(s.GrpcServer.Srv)

	return s, nil
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*callbacker_api.HealthResponse, error) {
	status := nats.DISCONNECTED
	if s.mqClient != nil {
		status = s.mqClient.Status()
	}
	return &callbacker_api.HealthResponse{
		Nats:      status.String(),
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}

func (s *Server) SendCallback(ctx context.Context, request *callbacker_api.SendRequest) (*emptypb.Empty, error) {
	callback := toStoreDto(request)
	_, err := s.store.Insert(ctx, []*store.CallbackData{callback})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ptrTo returns a pointer to the given value.
func ptrTo[T any](v T) *T {
	return &v
}
