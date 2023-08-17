package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/handler"
	apiHandler "github.com/bitcoin-sv/arc/api/handler"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/ordishs/gocore"
)

// This example does not use the configuration files or env variables,
// but demonstrates how to initialize the arc server in a completely custom way
func main() {

	// Set up a basic gocore logger
	logger := gocore.Log("arc", gocore.DEBUG)

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

					return fmt.Errorf("could not authenticate user")
				},
			},
		}),
	)

	// init BlockTx client
	blockTxClient := blocktx.NewClient(logger, "localhost:8021")
	grpcMessageSize, _ := gocore.Config().GetInt("grpc_message_size", 1e8)
	// add a single metamorph, with the BlockTx client we want to use
	txHandler, err := transactionHandler.NewMetamorph("localhost:8011", blockTxClient, grpcMessageSize)
	if err != nil {
		panic(err)
	}

	defaultPolicy, err := handler.GetDefaultPolicy()
	if err != nil {
		logger.Error(err)
		// this is a fatal error, we cannot start the server without a valid default policy
		panic(err)
	}

	// initialise the arc default api handler, with our txHandler and any handler options
	var handler api.HandlerInterface
	if handler, err = apiHandler.NewDefault(logger, txHandler, defaultPolicy); err != nil {
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
