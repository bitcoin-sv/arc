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
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/api/handler"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/bitcoin-sv/arc/pkg/metamorph"

	"github.com/labstack/echo-contrib/echoprometheus"
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

	e.Use(echomiddleware.RequestLoggerWithConfig(echomiddleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogError:    true,
		HandleError: true, // forwards error to the global error handler, so it can decide appropriate status code
		LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
			if v.Error == nil {
				logger.LogAttrs(context.Background(), slog.LevelInfo, "REQUEST",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
				)
			} else {
				logger.LogAttrs(context.Background(), slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}))

	// add prometheus metrics
	e.Use(echoprometheus.NewMiddleware("api"))

	// load the ARC handler from config
	// If you want to customize this for your own server, see examples dir
	// check the swagger definition against our requests
	handler.CheckSwagger(e)

	conn, err := metamorph.DialGRPC(arcConfig.Metamorph.DialAddr, arcConfig.PrometheusEndpoint, arcConfig.GrpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	var mqClient metamorph.MessageQueueClient
	natsClient, err := nats_connection.New(arcConfig.MessageQueue.URL, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
	}

	if arcConfig.MessageQueue.Streaming.Enabled {
		var opts []nats_jetstream.Option
		if arcConfig.MessageQueue.Streaming.FileStorage {
			opts = append(opts, nats_jetstream.WithFileStorage())
		}

		mqClient, err = nats_jetstream.New(natsClient, logger, []string{metamorph.SubmitTxTopic}, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create nats client: %v", err)
		}
	} else {
		mqClient = nats_core.New(natsClient, nats_core.WithLogger(logger))
	}

	metamorphClient := metamorph.NewClient(
		metamorph_api.NewMetaMorphAPIClient(conn),
		metamorph.WithMqClient(mqClient),
		metamorph.WithLogger(logger),
	)

	btcConn, err := blocktx.DialGRPC(arcConfig.Blocktx.DialAddr, arcConfig.PrometheusEndpoint, arcConfig.GrpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
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
		return nil, err
	}

	// Register the ARC API
	api.RegisterHandlers(e, apiHandler)

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

		mqClient.Shutdown()
	}, nil
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
