package handler

import (
	"log/slog"

	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/bitcoin-sv/arc/internal/api"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
)

// Server type carries the logger within it.
type Server struct {
	grpc_utils.GrpcServer

	handler api.ArcDefaultHandlerHealth
	logger  *slog.Logger
}

// NewServer will return a server instance with the logger stored within it.
func NewServer(logger *slog.Logger, handler api.ArcDefaultHandlerHealth, cfg grpc_utils.ServerConfig) (*Server, error) {
	logger = logger.With(slog.String("module", "server"))

	grpcServer, err := grpc_utils.NewGrpcServer(logger, cfg)
	if err != nil {
		return nil, err
	}

	s := &Server{
		GrpcServer: grpcServer,
		handler:    handler,
		logger:     logger,
	}

	// register health server endpoint
	grpc_health_v1.RegisterHealthServer(grpcServer.Srv, s)
	reflection.Register(s.GrpcServer.Srv)
	return s, nil
}
