package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/handler"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/config"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	apmecho "github.com/opentracing-contrib/echo"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-bitcoin"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func StartAPIServer(logger *slog.Logger) (func(), error) {
	// Set up a basic Echo router
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Recover returns a middleware which recovers from panics anywhere in the chain
	e.Use(echomiddleware.Recover())

	if opentracing.GlobalTracer() != nil {
		e.Use(apmecho.Middleware("arc"))
	}

	// Add CORS headers to the server - all request origins are allowed
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))

	// use the standard echo logger
	e.Use(echomiddleware.Logger())

	// load the ARC handler from config
	// If you want to customize this for your own server, see examples dir
	if err := LoadArcHandler(e, logger); err != nil {
		panic(err)
	}

	apiAddress := viper.GetString("api.address")
	if apiAddress == "" {
		panic("api.address not found in config")
	}
	// Serve HTTP until the world ends.
	go func() {
		logger.Info("Starting API server", slog.String("address", apiAddress))
		err := e.Start(apiAddress)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				logger.Info("API http server closed")
				return
			}

			logger.Error("Failed to start API server", slog.String("err", err.Error()))
			return
		}
	}()

	return func() {
		logger.Info("Shutting down api service")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			logger.Error("Failed to close API echo server", slog.String("err", err.Error()))
		}
	}, nil
}

func LoadArcHandler(e *echo.Echo, logger *slog.Logger) error {
	// check the swagger definition against our requests
	handler.CheckSwagger(e)

	// Check the security requirements

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

	isCentralisedMetamorph := false
	if viper.GetString("metamorph.db.mode") == "dynamodb" || viper.GetString("metamorph.db.mode") == "postgres" {
		isCentralisedMetamorph = true
	}

	txHandler, err := transactionHandler.NewMetamorph(addresses, bTx, grpcMessageSize, isCentralisedMetamorph)
	if err != nil {
		return err
	}

	var policy *bitcoin.Settings
	policy, err = getPolicyFromNode()
	if err != nil {
		policy, err = handler.GetDefaultPolicy()
		if err != nil {
			return err
		}
	}

	// TODO WithSecurityConfig(appConfig.Security)
	apiHandler, err := handler.NewDefault(logger, txHandler, policy)
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
