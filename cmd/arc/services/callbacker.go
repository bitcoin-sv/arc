package cmd

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/callbacker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func StartCallbacker(logger *slog.Logger, config *config.ArcConfig) (func(), error) {
	logger = logger.With(slog.String("service", "callbacker"))
	logger.Info("Starting")

	callbackSrv, err := callbacker.New(&http.Client{Timeout: 5 * time.Second}, logger)
	if err != nil {
		return nil, fmt.Errorf("callbacker failed: %v", err)
	}

	srvOpts := []callbacker.ServerOption{
		callbacker.WithLogger(logger.With(slog.String("module", "callbacker-server"))),
	}
	server := callbacker.NewServer(callbackSrv, srvOpts...)
	err = server.Serve(config.Callbacker.ListenAddr, config.GrpcMessageSize, config.PrometheusEndpoint)
	if err != nil {
		return nil, fmt.Errorf("GRPCServer failed: %v", err)
	}

	healthServer, err := StartHealthServerCallbacker(server, config.Callbacker.Health, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	stopFn := func() {
		logger.Info("Shutting down callbacker")

		server.Shutdown()
		callbackSrv.GracefulStop()

		healthServer.Stop()

		logger.Info("Shutted down")
	}

	logger.Info("Ready to work")
	return stopFn, nil
}

func StartHealthServerCallbacker(serv *callbacker.Server, healthConfig *config.HealthConfig, logger *slog.Logger) (*grpc.Server, error) {
	gs := grpc.NewServer()

	grpc_health_v1.RegisterHealthServer(gs, serv) // registration
	// register your own services
	reflection.Register(gs)

	listener, err := net.Listen("tcp", healthConfig.SeverDialAddr)
	if err != nil {
		return nil, err
	}

	go func() {
		logger.Info("GRPC health server listening", slog.String("address", healthConfig.SeverDialAddr))
		err = gs.Serve(listener)
		if err != nil {
			logger.Error("GRPC health server failed to serve", slog.String("err", err.Error()))
		}
	}()

	return gs, nil
}
