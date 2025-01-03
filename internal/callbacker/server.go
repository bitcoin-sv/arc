package callbacker

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
)

type Server struct {
	callbacker_api.UnimplementedCallbackerAPIServer
	grpc_opts.GrpcServer
	dispatcher *CallbackDispatcher
}

// NewServer will return a server instance
func NewServer(prometheusEndpoint string, maxMsgSize int, logger *slog.Logger, dispatcher *CallbackDispatcher, tracingConfig *config.TracingConfig) (*Server, error) {
	logger = logger.With(slog.String("module", "server"))

	grpcServer, err := grpc_opts.NewGrpcServer(logger, "callbacker", prometheusEndpoint, maxMsgSize, tracingConfig)
	if err != nil {
		return nil, err
	}

	s := &Server{
		GrpcServer: grpcServer,
		dispatcher: dispatcher,
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
			s.dispatcher.Dispatch(r.Url, &CallbackEntry{Token: r.Token, Data: dto}, r.AllowBatch)
		}
	}

	return nil, nil
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
