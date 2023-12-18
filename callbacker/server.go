package callbacker

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server type carries the logger within it.
type Server struct {
	callbacker_api.UnsafeCallbackerAPIServer
	logger     *slog.Logger
	callbacker *Callbacker
	grpcServer *grpc.Server
}

// NewServer will return a server instance with the logger stored within it.
func NewServer(logger *slog.Logger, c *Callbacker) *Server {
	return &Server{
		logger:     logger,
		callbacker: c,
	}
}

// StartGRPCServer function.
func (s *Server) StartGRPCServer(address string) error {
	var opts []grpc.ServerOption
	opts = append(opts,
		grpc.ChainStreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.ChainUnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	s.grpcServer = grpc.NewServer(tracing.AddGRPCServerOptions(opts)...)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("GRPC server failed to listen [%w]", err)
	}

	callbacker_api.RegisterCallbackerAPIServer(s.grpcServer, s)

	// Register reflection service on gRPC server.
	reflection.Register(s.grpcServer)

	s.logger.Info("GRPC server listening", slog.String("address", address))

	if err = s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("GRPC server failed [%w]", err)
	}

	return nil
}

func (s *Server) Shutdown() {
	s.logger.Info("Shutting down")
	s.grpcServer.Stop()
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*callbacker_api.HealthResponse, error) {
	return &callbacker_api.HealthResponse{
		Ok:        true,
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}

func (s *Server) RegisterCallback(ctx context.Context, callback *callbacker_api.Callback) (*callbacker_api.RegisterCallbackResponse, error) {
	if _, err := url.ParseRequestURI(callback.GetUrl()); err != nil {
		return nil, fmt.Errorf("invalid URL [%w]", err)
	}

	key, err := s.callbacker.AddCallback(ctx, callback)
	if err != nil {
		return nil, err
	}

	return &callbacker_api.RegisterCallbackResponse{
		Key: key,
	}, nil
}
