package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	middleware "github.com/oapi-codegen/echo-middleware"

	goscript "github.com/bitcoin-sv/bdk/module/gobdk/script"

	"github.com/bitcoin-sv/arc/config"
	apiHandler "github.com/bitcoin-sv/arc/internal/api/handler"
	apimocks "github.com/bitcoin-sv/arc/internal/api/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/node_client"
	tx_finder "github.com/bitcoin-sv/arc/internal/tx_finder"
	beefValidator "github.com/bitcoin-sv/arc/internal/validator/beef"
	defaultValidator "github.com/bitcoin-sv/arc/internal/validator/default"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/rpc_client"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

// This example does not use the configuration files or env variables,
// but demonstrates how to initialize the arc server in a completely custom way
func main() {
	// Set up a basic logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Set up a basic Echo router
	e := echo.New()

	// Add CORS headers to the server - all request origins are allowed
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))

	// ------------------------------------------------------------------------
	// This is the custom section, which does not use the appConfig
	//

	// check the swagger definition against our requests
	swagger := apiHandler.CheckSwagger(e)

	// Set a custom authentication handler
	e.Use(middleware.OapiRequestValidatorWithOptions(swagger,
		&middleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: func(_ context.Context, input *openapi3filter.AuthenticationInput) error {
					// in here you can add any kind of authentication check, like a database lookup on an blocktx_api-key
					if input.SecuritySchemeName != "BearerAuth" {
						return fmt.Errorf("security scheme %s != 'BearerAuth'", input.SecuritySchemeName)
					}

					authorizationHeader := input.RequestValidationInput.Request.Header.Get("Authorization")

					// Remove the "Bearer" prefix
					apiKey := strings.Replace(authorizationHeader, "Bearer ", "", 1)

					// don't do this in production
					if apiKey == "test-key" {
						return nil
					}

					return errors.New("could not authenticate user")
				},
			},
		}),
	)

	// Get app config
	arcConfig, err := config.Load("/path/to/custom/config/")
	if err != nil {
		panic(err)
	}

	// add a single metamorph, with the BlockTx client we want to use
	conn, err := grpc_utils.DialGRPC("localhost:8011", "", arcConfig.GrpcMessageSize, nil)
	if err != nil {
		panic(err)
	}

	metamorphClient := metamorph.NewClient(metamorph_api.NewMetaMorphAPIClient(conn))

	// add blocktx as MerkleRootsVerifier
	btcConn, err := grpc_utils.DialGRPC("localhost:8011", "", arcConfig.GrpcMessageSize, nil)
	if err != nil {
		panic(err)
	}

	blockTxClient := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(btcConn))

	network := arcConfig.Network
	genesisBlock := apiHandler.GenesisForkBlockRegtest
	switch arcConfig.Network {
	case "testnet":
		network = "test"
		genesisBlock = apiHandler.GenesisForkBlockTest
	case "mainnet":
		network = "main"
		genesisBlock = apiHandler.GenesisForkBlockMain
	default:
	}

	se := goscript.NewScriptEngine(network)

	pc := arcConfig.PeerRPC
	nc, err := rpc_client.NewRPCClient(pc.Host, pc.Port, pc.User, pc.Password)
	if err != nil {
		panic(err)
	}

	nodeClient, err := node_client.New(nc)
	if err != nil {
		panic(err)
	}

	wocClient := woc_client.New(arcConfig.API.WocMainnet)

	finder := tx_finder.New(metamorphClient, nodeClient, wocClient, logger)

	cachedFinder := tx_finder.NewCached(finder)
	dv := defaultValidator.New(
		arcConfig.API.DefaultPolicy,
		cachedFinder,
		se,
		genesisBlock,
	)

	// initialise the arc default api handler, with our txHandler and any handler options
	var handler api.ServerInterface

	chainTrackerMock := &apimocks.ChainTrackerMock{}
	bv := beefValidator.New(arcConfig.API.DefaultPolicy, chainTrackerMock, se, genesisBlock)
	defaultHandler, err := apiHandler.NewDefault(logger, metamorphClient, blockTxClient, arcConfig.API.DefaultPolicy, dv, bv)
	if err != nil {
		panic(err)
	}

	defaultHandler.StartUpdateCurrentBlockHeight()
	handler = defaultHandler

	serverCfg := grpc_utils.ServerConfig{
		PrometheusEndpoint: arcConfig.Prometheus.Endpoint,
		MaxMsgSize:         arcConfig.GrpcMessageSize,
		TracingConfig:      arcConfig.Tracing,
		Name:               "api",
	}

	server, err := apiHandler.NewServer(logger, defaultHandler, serverCfg)
	if err != nil {
		panic(fmt.Errorf("create GRPCServer failed: %w", err))
	}
	err = server.ListenAndServe(arcConfig.API.ListenAddr)
	if err != nil {
		panic(fmt.Errorf("serve GRPC server failed: %w", err))
	}

	// Register the ARC API
	// the arc handler registers routes under /v1/...
	api.RegisterHandlers(e, handler)
	// or with a base url => /mySubDir/v1/...
	// arc.RegisterHandlersWithBaseURL(e. blocktx_api, "/mySubDir")

	// Add the echo standard logger
	e.Use(echomiddleware.Logger())

	//
	// /custom section
	// ------------------------------------------------------------------------

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", "0.0.0.0", 8080)))
}
