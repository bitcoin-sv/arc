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

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/api"
	apiHandler "github.com/bitcoin-sv/arc/pkg/api/handler"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
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
				AuthenticationFunc: func(c context.Context, input *openapi3filter.AuthenticationInput) error {
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
	conn, err := metamorph.DialGRPC("localhost:8011", "", arcConfig.GrpcMessageSize, nil)
	if err != nil {
		panic(err)
	}

	metamorphClient := metamorph.NewClient(metamorph_api.NewMetaMorphAPIClient(conn))

	// add blocktx as MerkleRootsVerifier
	btcConn, err := blocktx.DialGRPC("localhost:8011", "", arcConfig.GrpcMessageSize, nil)
	if err != nil {
		panic(err)
	}

	blockTxClient := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(btcConn))

	// initialise the arc default api handler, with our txHandler and any handler options
	var handler api.ServerInterface
	if handler, err = apiHandler.NewDefault(logger, metamorphClient, blockTxClient, arcConfig.API.DefaultPolicy, nil); err != nil {
		panic(err)
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
