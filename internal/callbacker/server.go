package callbacker

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var ErrServerFailedToListen = errors.New("GRPC server failed to listen")

type Server struct {
	callbacker_api.UnimplementedCallbackerAPIServer
	callbacker CallbackerI
	grpcServer *grpc.Server
	logger     *slog.Logger
	cleanup    func()
}

type ServerOption func(s *Server)

func WithLogger(logger *slog.Logger) func(*Server) {
	return func(s *Server) {
		s.logger = logger
	}
}

// NewServer will return a server instance
func NewServer(callbacker CallbackerI, opts ...ServerOption) *Server {
	server := &Server{
		callbacker: callbacker,
		logger:     slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).With(slog.String("module", "server")),
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

func (s *Server) Serve(address string, grpcMessageSize int, prometheusEndpoint string) error {
	// LEVEL 0 - no security / no encryption

	srvMetrics, opts, cleanup, err := grpc_opts.GetGRPCServerOpts(s.logger, prometheusEndpoint, grpcMessageSize, "callbacker")
	if err != nil {
		return err
	}

	s.cleanup = cleanup

	grpcSrv := grpc.NewServer(opts...)
	srvMetrics.InitializeMetrics(grpcSrv)

	s.grpcServer = grpcSrv

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return ErrServerFailedToListen
	}

	callbacker_api.RegisterCallbackerAPIServer(grpcSrv, s)
	reflection.Register(s.grpcServer)

	go func() {
		s.logger.Info("GRPC server listening", slog.String("address", address))
		err = s.grpcServer.Serve(lis)
		if err != nil {
			s.logger.Error("GRPC server failed to serve", slog.String("err", err.Error()))
		}
	}()

	return nil
}

func (s *Server) GracefulStop() {
	s.logger.Info("Shutting down")

	s.grpcServer.GracefulStop()

	s.cleanup()
	s.logger.Info("Shutted down")
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*callbacker_api.HealthResponse, error) {
	return &callbacker_api.HealthResponse{
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}

func (s *Server) SendCallback(_ context.Context, request *callbacker_api.SendCallbackRequest) (*emptypb.Empty, error) {
	dto := toCallbackDto(request)
	for _, callbackEndpoint := range request.CallbackEndpoints {
		if callbackEndpoint.Url != "" {
			s.callbacker.Send(callbackEndpoint.Url, callbackEndpoint.Token, dto)
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
