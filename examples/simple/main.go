package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/pkg/api"
	apiHandler "github.com/bitcoin-sv/arc/pkg/api/handler"
	merklerootsverifier "github.com/bitcoin-sv/arc/pkg/api/merkle_roots_verifier"
	"github.com/bitcoin-sv/arc/pkg/api/transaction_handler"
	"github.com/labstack/echo/v4"
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

	// initialise the arc default api handler, with our txHandler and any handler options
	var handler api.ServerInterface
	if handler, err = apiHandler.NewDefault(logger, txHandler, merkleRootsVerifier, arcConfig.Api.DefaultPolicy, arcConfig.PeerRpc, arcConfig.Api); err != nil {
		panic(err)
	}

	// Register the ARC API
	// the arc handler registers routes under /v1/...
	api.RegisterHandlers(e, handler)
	// or with a base url => /mySubDir/v1/...
	// arc.RegisterHandlersWithBaseURL(e. blocktx_api, "/arc")

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", "0.0.0.0", 8080)))
}
