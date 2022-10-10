package main

import (
	"fmt"
	"os"

	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/mrz1836/go-logger"
	"github.com/taal/mapi"
	"github.com/taal/mapi/config"
	"github.com/taal/mapi/dictionary"
	"github.com/taal/mapi/handler"
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

	// Create an instance of our handler which satisfies the generated interface
	var opts []handler.Options
	opts = append(opts, handler.WithMinerID(appConfig.MinerID))
	if appConfig.RestClient != nil {
		handler.WithBitcoinRestServer(appConfig.RestClient.Server)
	}
	if appConfig.RPCClient != nil {
		handler.WithBitcoinRPCServer(appConfig.RPCClient)
	}
	api := handler.NewDefault(opts...)

	// This is how you set up a basic Echo router
	e := echo.New()
	// Log all requests
	e.Use(echomiddleware.Logger())
	// Use our validation middleware to check all requests against the
	// OpenAPI schema.
	e.Use(middleware.OapiRequestValidator(swagger))

	// We now register our petStore above as the handler for the interface
	mapi.RegisterHandlers(e, api)

	// And we serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", appConfig.Server.IPAddress, appConfig.Server.Port)))
}
