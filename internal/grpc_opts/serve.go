package grpc_opts

import (
	"errors"
	"log/slog"
	"net"

	"google.golang.org/grpc"
)

var ErrServerFailedToListen = errors.New("GRPC server failed to listen")

type GrpcServer struct {
	Srv *grpc.Server

	logger  *slog.Logger
	cleanup func()
}

func NewGrpcServer(logger *slog.Logger, name, prometheusEndpoint string, maxMsgSize int) (GrpcServer, error) {
	metrics, grpcOpts, cleanupFn, err := GetGRPCServerOpts(logger, prometheusEndpoint, maxMsgSize, name)
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
		return ErrServerFailedToListen
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
	s.logger.Info("Shutting down")

	s.Srv.GracefulStop()

	if s.cleanup != nil {
		s.cleanup()
	}

	s.logger.Info("Shutted down")
}
