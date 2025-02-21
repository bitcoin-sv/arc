package grpc_opts

import (
	"errors"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

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

func NewHealthServer(logger *slog.Logger, serv grpc_health_v1.HealthServer) *GrpcServer {
	grpcSrv := grpc.NewServer()

	grpc_health_v1.RegisterHealthServer(grpcSrv, serv)
	reflection.Register(grpcSrv)

	return &GrpcServer{
		Srv:    grpcSrv,
		logger: logger.With(slog.String("service", "health-server")),
	}
}

func ServeNewHealthServer(logger *slog.Logger, serv grpc_health_v1.HealthServer, address string) (*GrpcServer, error) {
	srv := NewHealthServer(logger, serv)

	if err := srv.ListenAndServe(address); err != nil {
		srv.GracefulStop()
		return nil, err
	}

	return srv, nil
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

	s.logger.Info("Shutdown complete")
}
