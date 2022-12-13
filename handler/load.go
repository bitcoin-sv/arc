package handler

import (
	"fmt"
	"os"

	arc "github.com/TAAL-GmbH/arc"
	"github.com/TAAL-GmbH/arc/client"
	"github.com/TAAL-GmbH/arc/config"
	"github.com/TAAL-GmbH/arc/dictionary"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/mrz1836/go-logger"
)

func LoadArcHandler(e *echo.Echo, appConfig *config.AppConfig) error {

	// check the swagger definition against our requests
	CheckSwagger(e)

	// Check the security requirements
	CheckSecurity(e, appConfig)

	// Create an instance of our handler which satisfies the generated interface
	var opts []client.Options

	// Add cachestore
	if appConfig.Cachestore.Engine == "redis" {
		opts = append(opts, client.WithRedis(appConfig.Redis))
	} else {
		// set up an in memory local cache, using free cache
		opts = append(opts, client.WithFreeCache())
	}

	// load the api, using the default handler
	c, err := client.New(opts...)
	if err != nil {
		return err
	}

	var api arc.HandlerInterface
	if api, err = NewDefault(c, WithSecurityConfig(appConfig.Security)); err != nil {
		return err
	}

	// Register the ARC API
	arc.RegisterHandlers(e, api)

	return nil
}

// CheckSwagger validates the request against the swagger definition
func CheckSwagger(e *echo.Echo) *openapi3.T {

	swagger, err := arc.GetSwagger()
	if err != nil {
		logger.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorLoadingSwaggerSpec), err.Error())
		os.Exit(1)
	}

	// Clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil
	// Clear out the security requirements, we check this ourselves
	swagger.Security = nil

	// Use our validation middleware to check all requests against the OpenAPI schema.
	e.Use(middleware.OapiRequestValidator(swagger))

	return swagger
}

// CheckSecurity validates a request against the configured security
func CheckSecurity(e *echo.Echo, appConfig *config.AppConfig) {

	if appConfig.Security != nil {
		if appConfig.Security.Type == config.SecurityTypeJWT {
			e.Pre(echomiddleware.JWTWithConfig(echomiddleware.JWTConfig{
				SigningKey: []byte(appConfig.Security.BearerKey),
				Claims:     &arc.JWTCustomClaims{},
			}))
		} else if appConfig.Security.Type == config.SecurityTypeCustom {
			e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(ctx echo.Context) error {
					_, err := appConfig.Security.CustomGetUser(ctx)
					if err != nil {
						ctx.Error(err)
					}
					return next(ctx)
				}
			})
		} else {
			panic(fmt.Errorf("unknown security type: %s", appConfig.Security.Type))
		}
	}
}
