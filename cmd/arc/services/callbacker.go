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

func StartCallbacker(logger *slog.Logger, appConfig *config.ArcConfig) (func(), error) {
	logger = logger.With(slog.String("service", "callbacker"))
	logger.Info("Starting")

	config := appConfig.Callbacker

	callbackSender, err := callbacker.NewSender(&http.Client{Timeout: 5 * time.Second}, logger)
	if err != nil {
		return nil, fmt.Errorf("callbacker failed: %v", err)
	}

	callbackDispatcher := callbacker.NewCallbackDispatcher(callbackSender, config.Pause)

	server := callbacker.NewServer(callbackDispatcher, callbacker.WithLogger(logger.With(slog.String("module", "server"))))
	err = server.Serve(config.ListenAddr, appConfig.GrpcMessageSize, appConfig.PrometheusEndpoint)
	if err != nil {
		return nil, fmt.Errorf("GRPCServer failed: %v", err)
	}

	healthServer, err := StartHealthServerCallbacker(server, config.Health, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	stopFn := func() {
		logger.Info("Shutting down callbacker")

		// dispose of dependencies in the correct order:
		// 1. server - ensure no new callbacks will be received
		// 2. dispatcher - ensure all already accepted callbacks are proccessed
		// 3. sender - finally, stop the sender as there are no callbacks left to send.
		server.GracefulStop()
		callbackDispatcher.GracefulStop()
		callbackSender.GracefulStop()

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
