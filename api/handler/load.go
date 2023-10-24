package handler

import (
	"fmt"
	"log"
	"net/url"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/dictionary"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/config"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func LoadArcHandler(e *echo.Echo, logger utils.Logger) error {

	// check the swagger definition against our requests
	CheckSwagger(e)

	// Check the security requirements
	//CheckSecurity(e, appConfig)

	addresses := viper.GetString("metamorph.dialAddr")
	if addresses == "" {
		return fmt.Errorf("metamorph.dialAddr not found in config")
	}

	blocktxAddress := viper.GetString("blocktx.dialAddr")
	if blocktxAddress == "" {
		return fmt.Errorf("blocktx.dialAddr not found in config")
	}

	blockTxLogger, err := config.NewLogger()
	if err != nil {
		return fmt.Errorf("failed to create new logger: %v", err)
	}

	conn, err := blocktx.DialGRPC(blocktxAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	bTx := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(conn), blocktx.WithLogger(blockTxLogger))

	grpcMessageSize := viper.GetInt("grpcMessageSize")
	if grpcMessageSize == 0 {
		return fmt.Errorf("grpcMessageSize not found in config")
	}

	txHandler, err := transactionHandler.NewMetamorph(addresses, bTx, grpcMessageSize, logger)
	if err != nil {
		return err
	}

	var policy *bitcoin.Settings
	policy, err = getPolicyFromNode()
	if err != nil {
		policy, err = GetDefaultPolicy()
		if err != nil {
			return err
		}
	}

	// TODO WithSecurityConfig(appConfig.Security)
	apiHandler, err := NewDefault(logger, txHandler, policy)
	if err != nil {
		return err
	}

	// Register the ARC API
	api.RegisterHandlers(e, apiHandler)

	return nil
}

func getPolicyFromNode() (*bitcoin.Settings, error) {
	peerRpcPassword := viper.GetString("peerRpc.password")
	if peerRpcPassword == "" {
		return nil, errors.Errorf("setting peerRpc.password not found")
	}

	peerRpcUser := viper.GetString("peerRpc.user")
	if peerRpcUser == "" {
		return nil, errors.Errorf("setting peerRpc.user not found")
	}

	peerRpcHost := viper.GetString("peerRpc.host")
	if peerRpcHost == "" {
		return nil, errors.Errorf("setting peerRpc.host not found")
	}

	peerRpcPort := viper.GetInt("peerRpc.port")
	if peerRpcPort == 0 {
		return nil, errors.Errorf("setting peerRpc.port not found")
	}

	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpcUser, peerRpcPassword, peerRpcHost, peerRpcPort))
	if err != nil {
		return nil, errors.Errorf("failed to parse rpc URL: %v", err)
	}

	// connect to bitcoin node and get the settings
	b, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		return nil, fmt.Errorf("error connecting to peer: %v", err)
	}

	settings, err := b.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("error getting settings from peer: %v", err)
	}

	return &settings, nil
}

func GetDefaultPolicy() (*bitcoin.Settings, error) {
	defaultPolicy := &bitcoin.Settings{}

	err := viper.UnmarshalKey("api.defaultPolicy", defaultPolicy)
	if err != nil {
		return nil, err
	}

	return defaultPolicy, nil
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
