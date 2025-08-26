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

type Dispatcher interface {
	Dispatch(url string, dto *CallbackEntry)
}

type Server struct {
	callbacker_api.UnimplementedCallbackerAPIServer
	grpc_utils.GrpcServer
	dispatcher Dispatcher
	store      store.CallbackStore
	mqClient   mq.MessageQueueClient
	logger     *slog.Logger
}

// NewServer will return a server instance
func NewServer(logger *slog.Logger, dispatcher Dispatcher, callbackerStore store.CallbackStore, mqClient mq.MessageQueueClient, cfg grpc_utils.ServerConfig) (*Server, error) {
	grpcServer, err := grpc_utils.NewGrpcServer(logger, cfg)
	if err != nil {
		return nil, err
	}

	s := &Server{
		GrpcServer: grpcServer,
		dispatcher: dispatcher,
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
	status := nats.DISCONNECTED.String()
	if s.mqClient != nil {
		status = s.mqClient.Status()
	}
	return &callbacker_api.HealthResponse{
		Nats:      status,
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}

func (s *Server) SendCallback(_ context.Context, request *callbacker_api.SendCallbackRequest) (*emptypb.Empty, error) {
	dto := toCallbackDto(request)
	for _, r := range request.CallbackRoutings {
		if r.Url != "" {
			s.dispatcher.Dispatch(r.Url, &CallbackEntry{Token: r.Token, Data: dto, AllowBatch: r.AllowBatch})
		}
	}

	return nil, nil
}

func (s *Server) UpdateInstances(ctx context.Context, request *callbacker_api.UpdateInstancesRequest) (*emptypb.Empty, error) {
	rowsAffected, err := s.store.DeleteURLMappingsExcept(ctx, request.Instances)
	if err != nil {
		return nil, err
	}

	if rowsAffected > 0 {
		s.logger.Info("URL mappings deleted", slog.Int64("items", rowsAffected))
	}

	return &emptypb.Empty{}, nil
}

func toCallbackDto(r *callbacker_api.SendCallbackRequest) *Callback {
	dto := Callback{
		TxID:      r.Txid,
		TxStatus:  r.Status.String(),
		Timestamp: time.Now().UTC(),
	}

	if r.BlockHash != "" {
		dto.BlockHash = ptrTo(r.BlockHash)
		dto.BlockHeight = ptrTo(r.BlockHeight)
	}

	if r.MerklePath != "" {
		dto.MerklePath = ptrTo(r.MerklePath)
	}

	if r.ExtraInfo != "" {
		dto.ExtraInfo = ptrTo(r.ExtraInfo)
	}

	if len(r.CompetingTxs) > 0 {
		dto.CompetingTxs = r.CompetingTxs
	}

	return &dto
}

// ptrTo returns a pointer to the given value.
func ptrTo[T any](v T) *T {
	return &v
}
