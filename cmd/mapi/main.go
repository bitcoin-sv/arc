package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/mrz1836/go-logger"
	"github.com/taal/mapi"
	"github.com/taal/mapi/client"
	"github.com/taal/mapi/config"
	"github.com/taal/mapi/dictionary"
	"github.com/taal/mapi/handler"
	"github.com/taal/mapi/models"
)

func main() {

	// Load the Application Configuration
	appConfig, err := config.Load("")
	if err != nil {
		logger.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorLoadingConfig), err.Error())
		return
	}

	var swagger *openapi3.T
	swagger, err = mapi.GetSwagger()
	if err != nil {
		logger.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorLoadingSwaggerSpec), err.Error())
		os.Exit(1)
	}

	// Clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	// Set up a basic Echo router
	e := echo.New()
	// Log all requests
	e.Use(echomiddleware.Logger())

	// Use our validation middleware to check all requests against the OpenAPI schema.
	// TODO implement Bearer JWT security check & custom interface
	if appConfig.Security == nil || appConfig.Security.Provider == "" {
		e.Use(middleware.OapiRequestValidatorWithOptions(swagger, &middleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
					fmt.Println(">>>> NOT Authenticating request")
					return nil
				},
			},
		}))
	} else {
		e.Use(middleware.OapiRequestValidator(swagger))
	}

	// Add CORS headers to the server - all request origins are allowed
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))

	// Register the MAPI API
	mapi.RegisterHandlers(e, loadMapiHandler(appConfig))

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", appConfig.Server.IPAddress, appConfig.Server.Port)))
}

func loadMapiHandler(appConfig *config.AppConfig) mapi.ServerInterface {
	// Create an instance of our handler which satisfies the generated interface
	var opts []client.Options
	opts = append(opts, client.WithMinerID(appConfig.MinerID))
	opts = append(opts, client.WithMigrateModels(models.BaseModels))

	// MAPI does not work without an RPC compatible client
	if appConfig.RPCClient == nil {
		panic("rpc client configuration missing")
	}
	client.WithBitcoinRPCServer(appConfig.RPCClient)

	// Add datastore
	// by default an SQLite database will be used in ./mapi.db if no datastore options are set in config
	if appConfig.Datastore != nil {
		client.WithDatastore(appConfig.Datastore)
	}

	// load the api, using the default handler
	c, err := client.New(opts...)
	if err != nil {
		panic(err.Error())
	}

	var api mapi.HandlerInterface
	if api, err = handler.NewDefault(c); err != nil {
		panic(err.Error())
	}

	return api
}
