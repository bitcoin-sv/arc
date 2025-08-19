package grpc_utils

import (
	"errors"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"

	"github.com/bitcoin-sv/arc/config"
)

var ErrServerFailedToListen = errors.New("GRPC server failed to listen")

type GrpcServer struct {
	Srv *grpc.Server

	logger  *slog.Logger
	cleanup func()
}

type ServerConfig struct {
	PrometheusEndpoint string
	MaxMsgSize         int
	TracingConfig      *config.TracingConfig
	Name               string
}

func NewGrpcServer(logger *slog.Logger, cfg ServerConfig) (GrpcServer, error) {
	metrics, grpcOpts, cleanupFn, err := GetGRPCServerOpts(logger, cfg)
	if err != nil {
		return GrpcServer{}, err
	}

	grpcSrv := grpc.NewServer(grpcOpts...)

	metrics.InitializeMetrics(grpcSrv)

	return GrpcServer{
		Srv:     grpcSrv,
		logger:  logger,
		cleanup: cleanupFn,
	}, nil
}

func (s *GrpcServer) ListenAndServe(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return errors.Join(ErrServerFailedToListen, fmt.Errorf("address %s: %w", address, err))
	}

	go func() {
		s.logger.Info("GRPC server listening", slog.String("address", address))
		err = s.Srv.Serve(listener)
		if err != nil {
			s.logger.Error("GRPC server failed to serve", slog.String("err", err.Error()))
		}
	}()

	return nil
}

func (s *GrpcServer) GracefulStop() {
	s.logger.Info("Shutting down gRPC server")

	s.Srv.GracefulStop()

	if s.cleanup != nil {
		s.cleanup()
	}

	s.logger.Info("Shutdown gRPC server complete")
}
