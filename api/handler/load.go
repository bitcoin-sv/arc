package handler

import (
	"fmt"
	"log"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/dictionary"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

func LoadArcHandler(e *echo.Echo, logger utils.Logger) error {

	// check the swagger definition against our requests
	CheckSwagger(e)

	// Check the security requirements
	//CheckSecurity(e, appConfig)

	addresses, found := gocore.Config().Get("metamorphAddresses") //, "localhost:8001")
	if !found {
		return fmt.Errorf("metamorphAddresses not found in config")
	}

	blocktxAddress, _ := gocore.Config().Get("blocktxAddress") //, "localhost:8001")
	bTx := blocktx.NewClient(logger, blocktxAddress)

	txHandler, err := transactionHandler.NewMetamorph(addresses, bTx)
	if err != nil {
		return err
	}

	var apiHandler api.HandlerInterface
	// TODO WithSecurityConfig(appConfig.Security)
	if apiHandler, err = NewDefault(logger, txHandler); err != nil {
		return err
	}

	// Register the ARC API
	api.RegisterHandlers(e, apiHandler)

	return nil
}

// CheckSwagger validates the request against the swagger definition
func CheckSwagger(e *echo.Echo) *openapi3.T {

	swagger, err := api.GetSwagger()
	if err != nil {
		log.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorLoadingSwaggerSpec), err.Error())
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

/*
// CheckSecurity validates a request against the configured security
func CheckSecurity(e *echo.Echo, appConfig *config.AppConfig) {

	if appConfig.Security != nil {
		if appConfig.Security.Type == config.SecurityTypeJWT {
			e.Pre(echomiddleware.JWTWithConfig(echomiddleware.JWTConfig{
				SigningKey: []byte(appConfig.Security.BearerKey),
				Claims:     &blocktx_api.JWTCustomClaims{},
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
*/
