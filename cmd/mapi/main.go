package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/config"
	"github.com/TAAL-GmbH/mapi/dictionary"
	"github.com/TAAL-GmbH/mapi/handler"
	"github.com/TAAL-GmbH/mapi/models"
	"github.com/TAAL-GmbH/mapi/server"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/mrz1836/go-logger"
	"github.com/ordishs/go-bitcoin"
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

	// TODO should this be activated?
	//e.Use(middleware.OapiRequestValidator(swagger))

	// Use our validation middleware to check all requests against the OpenAPI schema.
	if appConfig.Security != nil {
		if appConfig.Security.Provider == "BearerAuth" {
			e.Use(echomiddleware.JWTWithConfig(echomiddleware.JWTConfig{
				SigningKey: []byte(appConfig.Security.BearerKey),
				Claims:     &server.JWTCustomClaims{},
			}))
		} else if appConfig.Security.Provider == "apiKey" {
			// todo
		}
	}

	// Add CORS headers to the server - all request origins are allowed
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))

	TEMPTEMPGenerateTestToken(err, appConfig)

	mapiClient, api := loadMapiHandler(appConfig)
	// TODO mapi after hook for response

	// Register the MAPI API
	mapi.RegisterHandlers(e, api)

	// Add the MAPI specific logger, logs to DB
	e.Pre(handler.NewLogger(mapiClient, appConfig))

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", appConfig.Server.IPAddress, appConfig.Server.Port)))
}

func TEMPTEMPGenerateTestToken(err error, appConfig *config.AppConfig) {
	// TEMP TEMP TEMP
	claims := &server.JWTCustomClaims{
		ClientID:       "test",
		Name:           "Test User",
		Admin:          false,
		StandardClaims: jwt.StandardClaims{},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	var signedToken string
	signedToken, err = token.SignedString([]byte(appConfig.Security.BearerKey))
	if err != nil {
		panic(err)
	}
	fmt.Println(signedToken)
}

func loadMapiHandler(appConfig *config.AppConfig) (client.Interface, mapi.ServerInterface) {
	// Create an instance of our handler which satisfies the generated interface
	var opts []client.Options
	opts = append(opts, client.WithMinerID(appConfig.MinerID))
	opts = append(opts, client.WithMigrateModels(models.BaseModels))

	// add custom node configuration, overwriting the default localhost node config
	if appConfig.Nodes != nil {
		for _, nodeConfig := range appConfig.Nodes {
			node, err := bitcoin.New(nodeConfig.Host, nodeConfig.Port, nodeConfig.User, nodeConfig.Password, nodeConfig.UseSSL)
			if err != nil {
				panic(err)
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
		panic(err)
	}

	// run the custom migrations for all active models
	activeModels := c.Models()
	if len(activeModels) > 0 {
		// will run the custom model Migrate() method for all models
		for _, model := range activeModels {
			model.(models.ModelInterface).SetOptions(models.WithClient(c))
			if err = model.(models.ModelInterface).Migrate(c.Datastore()); err != nil {
				panic(err)
			}
		}
	}

	/*
		_policy := models.NewPolicy(models.WithClient(c), models.New())
		_policy.Fees = models.Fees{{
			FeeType: "standard",
			MiningFee: mapi.FeeAmount{
				Satoshis: 5,
				Bytes:    1000,
			},
			RelayFee: mapi.FeeAmount{
				Satoshis: 3,
				Bytes:    1000,
			},
		}, {
			FeeType: "data",
			MiningFee: mapi.FeeAmount{
				Satoshis: 17,
				Bytes:    1000,
			},
			RelayFee: mapi.FeeAmount{
				Satoshis: 2,
				Bytes:    1000,
			},
		}}
		//_policy.ClientID = "test"
		_policy.Metadata = models.Metadata{
			"test":  "test",
			"test2": "test2",
		}
		err = _policy.Save(context.Background())
		fmt.Println(err)
	*/

	var api mapi.HandlerInterface
	if api, err = handler.NewDefault(c, handler.WithSecurityConfig(appConfig.Security)); err != nil {
		panic(err.Error())
	}

	return c, api
}
