package callbacker

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
)

type Server struct {
	callbacker_api.UnimplementedCallbackerAPIServer
	grpc_utils.GrpcServer
	dispatcher Dispatcher
	store      store.CallbackStore
}

// NewServer will return a server instance
func NewServer(logger *slog.Logger, dispatcher Dispatcher, callbackerStore store.CallbackStore, cfg grpc_utils.ServerConfig) (*Server, error) {
	grpcServer, err := grpc_utils.NewGrpcServer(logger, cfg)
	if err != nil {
		return nil, err
	}

	s := &Server{
		GrpcServer: grpcServer,
		dispatcher: dispatcher,
		store:      callbackerStore,
	}

	callbacker_api.RegisterCallbackerAPIServer(s.GrpcServer.Srv, s)
	reflection.Register(s.GrpcServer.Srv)

	return s, nil
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*callbacker_api.HealthResponse, error) {
	return &callbacker_api.HealthResponse{
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

func (s *Server) DeleteURLMapping(_ context.Context, request *callbacker_api.DeleteURLMappingRequest) (*callbacker_api.DeleteURLMappingResponse, error) {
	rowsAffected, err := s.store.DeleteURLMapping(context.Background(), request.Instance)
	if err != nil {
		return nil, err
	}

	return &callbacker_api.DeleteURLMappingResponse{Rows: rowsAffected}, nil
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
