package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/nats_mq"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/api/handler"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/bitcoin-sv/arc/pkg/metamorph/async"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/ordishs/go-bitcoin"
	"github.com/pkg/errors"
)

func StartAPIServer(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	// Set up a basic Echo router
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Recover returns a middleware which recovers from panics anywhere in the chain
	e.Use(echomiddleware.Recover())

	// Add CORS headers to the server - all request origins are allowed
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))

	// use the standard echo logger
	e.Use(echomiddleware.Logger())

	// load the ARC handler from config
	// If you want to customize this for your own server, see examples dir
	if err := LoadArcHandler(e, logger, arcConfig); err != nil {
		return nil, err
	}

	// Serve HTTP until the world ends.
	go func() {
		logger.Info("Starting API server", slog.String("address", arcConfig.Api.Address))
		err := e.Start(arcConfig.Api.Address)
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

func LoadArcHandler(e *echo.Echo, logger *slog.Logger, arcConfig *config.ArcConfig) error {
	// check the swagger definition against our requests
	handler.CheckSwagger(e)

	conn, err := metamorph.DialGRPC(arcConfig.Metamorph.DialAddr, arcConfig.PrometheusEndpoint, arcConfig.GrpcMessageSize)
	if err != nil {
		return fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	natsClient, err := nats_mq.NewNatsClient(arcConfig.QueueURL)
	if err != nil {
		return fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.QueueURL, err)
	}

	mq := async.NewNatsMQClient(natsClient)

	metamorphClient := metamorph.NewClient(
		metamorph_api.NewMetaMorphAPIClient(conn),
		metamorph.WithMqClient(mq),
		metamorph.WithLogger(logger),
	)

	btcConn, err := blocktx.DialGRPC(arcConfig.Blocktx.DialAddr, arcConfig.PrometheusEndpoint, arcConfig.GrpcMessageSize)
	if err != nil {
		return fmt.Errorf("failed to connect to metamorph server: %v", err)
	}
	blockTxClient := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(btcConn))

	var policy *bitcoin.Settings
	policy, err = getPolicyFromNode(arcConfig.PeerRpc)
	if err != nil {
		policy = arcConfig.Api.DefaultPolicy
	}

	// TODO: WithSecurityConfig(appConfig.Security)
	apiOpts := []handler.Option{
		handler.WithCallbackUrlRestrictions(arcConfig.Metamorph.RejectCallbackContaining),
	}

	apiHandler, err := handler.NewDefault(logger, metamorphClient, blockTxClient, policy, arcConfig.PeerRpc, arcConfig.Api, apiOpts...)
	if err != nil {
		return err
	}

	// Register the ARC API
	api.RegisterHandlers(e, apiHandler)

	return nil
}

func getPolicyFromNode(peerRpcConfig *config.PeerRpcConfig) (*bitcoin.Settings, error) {
	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpcConfig.User, peerRpcConfig.Password, peerRpcConfig.Host, peerRpcConfig.Port))
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
