package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/bitcoin-sv/arc/config"
	apiHandler "github.com/bitcoin-sv/arc/internal/api/handler"
	apiHandlerMocks "github.com/bitcoin-sv/arc/internal/api/handler/mocks"
	merklerootsverifier "github.com/bitcoin-sv/arc/internal/api/merkle_roots_verifier"
	"github.com/bitcoin-sv/arc/internal/api/transaction_handler"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/pkg/api"
)

func main() {
	// Set up a basic Echo router
	e := echo.New()

	// Get app config
	arcConfig, err := config.Load()
	if err != nil {
		panic(err)
	}

	// add a single bitcoin node
	txHandler, err := transaction_handler.NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		panic(err)
	}

	// add merkle roots verifier
	merkleRootsVerifier := merklerootsverifier.NewAllowAllVerifier()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dv := &apiHandlerMocks.DefaultValidatorMock{}
	bv := &apiHandlerMocks.BeefValidatorMock{}

	// initialise the arc default api handler, with our txHandler and any handler options
	var handler api.ServerInterface
	defaultHandler, err := apiHandler.NewDefault(logger, txHandler, merkleRootsVerifier, arcConfig.API.DefaultPolicy, dv, bv)
	if err != nil {
		panic(err)
	}

	defaultHandler.StartUpdateCurrentBlockHeight()

	serverCfg := grpc_utils.ServerConfig{
		PrometheusEndpoint: arcConfig.Prometheus.Endpoint,
		MaxMsgSize:         arcConfig.GrpcMessageSize,
		TracingConfig:      arcConfig.Tracing,
		Name:               "api",
	}

	server, err := apiHandler.NewServer(logger, defaultHandler, serverCfg)
	if err != nil {
		panic(fmt.Errorf("create GRPCServer failed: %v", err))
	}
	err = server.ListenAndServe(arcConfig.API.ListenAddr)
	if err != nil {
		panic(fmt.Errorf("serve GRPC server failed: %v", err))
	}

	handler = defaultHandler

	// Register the ARC API
	// the arc handler registers routes under /v1/...
	api.RegisterHandlers(e, handler)
	// or with a base url => /mySubDir/v1/...
	// arc.RegisterHandlersWithBaseURL(e. blocktx_api, "/arc")

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", "0.0.0.0", 8080)))
}
