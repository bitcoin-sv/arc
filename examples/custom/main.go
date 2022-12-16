package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/TAAL-GmbH/arc/api"
	handler2 "github.com/TAAL-GmbH/arc/api/handler"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// This example does not use the configuration files or env variables,
// but demonstrates how to initialize the arc server in a completely custom way
func main() {

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
	swagger := handler2.CheckSwagger(e)

	// Set a custom authentication handler
	e.Use(middleware.OapiRequestValidatorWithOptions(swagger,
		&middleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: func(c context.Context, input *openapi3filter.AuthenticationInput) error {
					// in here you can add any kind of authentication check, like a database lookup on an blocktx_api-key
					if input.SecuritySchemeName != "BearerAuth" {
						return fmt.Errorf("security scheme %s != 'BearerAuth'", input.SecuritySchemeName)
					}

					apiKey := input.RequestValidationInput.Request.Header.Get("X-API-KEY")
					// don't do this in production
					if apiKey == "test-key" {
						return nil
					}

					return fmt.Errorf("could not authenticate user")
				},
			},
		}),
	)

	// add a single bitcoin node
	transactionHandler, err := transactionHandler.NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		panic(err)
	}

	// initialise the arc default blocktx_api handler, with our transactionHandler and any handler options
	var apiHandler api.HandlerInterface
	if apiHandler, err = handler2.NewDefault(transactionHandler); err != nil {
		panic(err)
	}

	// Register the ARC API
	// the arc handler registers routes under /arc/v1/...
	api.RegisterHandlers(e, apiHandler)
	// or with a base url => /mySubDir/arc/v1/...
	// arc.RegisterHandlersWithBaseURL(e. blocktx_api, "/mySubDir")

	// Add the echo standard logger
	e.Use(echomiddleware.Logger())

	//
	// /custom section
	// ------------------------------------------------------------------------

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", "0.0.0.0", 8080)))
}
