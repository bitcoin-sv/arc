package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	middleware "github.com/oapi-codegen/echo-middleware"

	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/bitcoin-sv/arc/internal/api/handler"
	"github.com/bitcoin-sv/arc/pkg/api"
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
	swagger := handler.CheckSwagger(e)

	// Set a custom authentication handler
	e.Use(middleware.OapiRequestValidatorWithOptions(swagger,
		&middleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: func(_ context.Context, input *openapi3filter.AuthenticationInput) error {
					// in here you can add any kind of authentication check, like a database lookup on an blocktx_api-key
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

	//
	// initialise our Custom arc handler
	// which also initialises the transaction_handler and datastore etc.
	//
	apiHandler, err := NewCustomHandler()
	if err != nil {
		panic(err)
	}

	// Register the ARC API
	// the arc handler registers routes under /arc/v2/...
	api.RegisterHandlers(e, apiHandler)
	// or with a base url => /mySubDir/arc/v2/...
	// arc.RegisterHandlersWithBaseURL(e. blocktx_api, "/mySubDir")

	// Add the echo standard logger
	e.Use(echomiddleware.Logger())

	//
	// /custom section
	// ------------------------------------------------------------------------

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", "0.0.0.0", 8080)))
}
