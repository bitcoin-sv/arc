package handler

import (
	"fmt"
	"os"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/config"
	"github.com/TAAL-GmbH/mapi/dictionary"
	"github.com/TAAL-GmbH/mapi/models"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/mrz1836/go-logger"
	"github.com/ordishs/go-bitcoin"
)

func LoadMapiHandler(e *echo.Echo, appConfig *config.AppConfig) error {

	// check the swagger definition against our requests
	CheckSwagger(e)

	// Check the security requirements
	CheckSecurity(e, appConfig)

	// Create an instance of our handler which satisfies the generated interface
	var opts []client.Options
	opts = append(opts, client.WithMinerID(appConfig.MinerID))
	opts = append(opts, client.WithMigrateModels(models.BaseModels))

	// add custom node configuration, overwriting the default localhost node config
	if appConfig.Nodes != nil {
		for _, nodeConfig := range appConfig.Nodes {
			node, err := bitcoin.New(nodeConfig.Host, nodeConfig.Port, nodeConfig.User, nodeConfig.Password, nodeConfig.UseSSL)
			if err != nil {
				return err
			}
			opts = append(opts, client.WithNode(node))
		}
	}

	// Add datastore
	// by default an SQLite database will be used in ./mapi.db if no datastore options are set in config
	if appConfig.Datastore != nil {
		opts = append(opts, client.WithDatastore(appConfig.Datastore))
	}

	// load the api, using the default handler
	c, err := client.New(opts...)
	if err != nil {
		return err
	}

	// run the custom migrations for all active models
	activeModels := c.Models()
	if len(activeModels) > 0 {
		// will run the custom model Migrate() method for all models
		for _, model := range activeModels {
			model.(models.ModelInterface).SetOptions(models.WithClient(c))
			if err = model.(models.ModelInterface).Migrate(c.Datastore()); err != nil {
				return err
			}
		}
	}

	var api mapi.HandlerInterface
	if api, err = NewDefault(c, WithSecurityConfig(appConfig.Security)); err != nil {
		return err
	}

	// Register the MAPI API
	mapi.RegisterHandlers(e, api)

	// Add the MAPI specific logger, logs to DB
	e.Pre(NewLogger(c, appConfig))

	TEMPTEMPGenerateTestToken(appConfig)

	return nil
}

func TEMPTEMPGenerateTestToken(appConfig *config.AppConfig) {
	// TEMP TEMP TEMP
	claims := &mapi.JWTCustomClaims{
		ClientID:       "test",
		Name:           "Test User",
		Admin:          false,
		StandardClaims: jwt.StandardClaims{},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	var signedToken string
	signedToken, err := token.SignedString([]byte(appConfig.Security.BearerKey))
	if err != nil {
		panic(err)
	}
	fmt.Println(signedToken)
}

// CheckSwagger validates the request against the swagger definition
func CheckSwagger(e *echo.Echo) *openapi3.T {

	swagger, err := mapi.GetSwagger()
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
			e.Use(echomiddleware.JWTWithConfig(echomiddleware.JWTConfig{
				SigningKey: []byte(appConfig.Security.BearerKey),
				Claims:     &mapi.JWTCustomClaims{},
			}))
		} else if appConfig.Security.Type == config.SecurityTypeCustom {
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
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
