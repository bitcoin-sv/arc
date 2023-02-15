package handler

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/api/dictionary"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

// NodePolicy and Fees are global variables that hold the node policy and required fees
// They are used in validation of transactions and can be overwritten if including in an existing setup
var NodePolicy = &api.NodePolicy{}
var Fees = &[]api.Fee{}

func LoadArcHandler(e *echo.Echo, l utils.Logger) error {

	// check the swagger definition against our requests
	CheckSwagger(e)

	// Check the security requirements
	//CheckSecurity(e, appConfig)

	err := loadPolicyAndFees(l)
	if err != nil {
		return fmt.Errorf("error loading policy: %v", err)
	}

	addresses, found := gocore.Config().Get("metamorphAddresses") //, "localhost:8001")
	if !found {
		return fmt.Errorf("metamorphAddresses not found in config")
	}

	blocktxAddress, _ := gocore.Config().Get("blocktxAddress") //, "localhost:8001")
	bTx := blocktx.NewClient(l, blocktxAddress)

	txHandler, err := transactionHandler.NewMetamorph(addresses, bTx)
	if err != nil {
		return err
	}

	var apiHandler api.HandlerInterface
	// TODO WithSecurityConfig(appConfig.Security)
	if apiHandler, err = NewDefault(txHandler); err != nil {
		return err
	}

	// Register the ARC API
	api.RegisterHandlers(e, apiHandler)

	return nil
}

func loadPolicyAndFees(logger utils.Logger) error {
	defaultFees, found := gocore.Config().Get("defaultFees")
	if found && defaultFees != "" {
		if err := json.Unmarshal([]byte(defaultFees), &Fees); err != nil {
			// this is a fatal error, we cannot start the server without valid default fees
			return fmt.Errorf("error unmarshalling defaultFees: %v", err)
		}
	}

	defaultPolicy, found := gocore.Config().Get("defaultPolicy")
	if found && defaultPolicy != "" {
		if err := json.Unmarshal([]byte(defaultPolicy), &NodePolicy); err != nil {
			// this is a fatal error, we cannot start the server without a valid default policy
			return fmt.Errorf("error unmarshalling defaultPolicy: %v", err)
		}
	}

	// try to get the policy from one of the peers
	peerCount, _ := gocore.Config().GetInt("peerCount")
	if peerCount > 0 {
		for i := 1; i <= peerCount; i++ {
			bitcoinRpc, err, rpcFound := gocore.Config().GetURL(fmt.Sprintf("peer_%d_rpc", i))
			if err == nil && rpcFound {
				// connect to bitcoin node and get the settings
				var policy *api.NodePolicy
				if policy, err = api.DoRPCRequest(bitcoinRpc, "getsettings", nil); err != nil {
					logger.Errorf("error getting policy from peer: %v", err)
				} else {
					NodePolicy = policy
					logger.Infof("Got policy from peer: %s", bitcoinRpc.Host)
					break
				}
			}
		}
	}

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
