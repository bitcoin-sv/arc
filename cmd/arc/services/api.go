package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/ordishs/go-bitcoin"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/config"
	apiHandler "github.com/bitcoin-sv/arc/internal/api/handler"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	arc_logger "github.com/bitcoin-sv/arc/internal/logger"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/internal/node_client"
	tx_finder "github.com/bitcoin-sv/arc/internal/tx_finder"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/pkg/tracing"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

func StartAPIServer(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	logger = logger.With(slog.String("service", "api"))
	logger.Info("Starting")
	var (
		mqClient   mq.MessageQueueClient
		echoServer *echo.Echo
		err        error
	)

	echoServer = setAPIEcho(logger, arcConfig.API)

	// load the ARC handler from config
	// If you want to customize this for your own server, see examples dir
	// check the swagger definition against our requests
	apiHandler.CheckSwagger(echoServer)

	shutdownFns := make([]func(), 0)
	stopFn := func() {
		logger.Info("Shutting down api")
		disposeAPI(logger, echoServer, mqClient, shutdownFns)
		logger.Info("Shutdown complete")
	}

	connOpts := []nats_connection.Option{nats_connection.WithMaxReconnects(-1)}

	mqClient, err = mq.NewMqClient(logger, arcConfig.MessageQueue, []nats_jetstream.Option{}, connOpts)
	if err != nil {
		stopFn()
		return nil, err
	}

	mtmOpts := []func(*metamorph.Metamorph){
		metamorph.WithMqClient(mqClient),
		metamorph.WithLogger(logger),
	}

	apiOpts := []apiHandler.Option{
		apiHandler.WithCallbackURLRestrictions(arcConfig.Metamorph.RejectCallbackContaining),
		apiHandler.WithCacheExpiryTime(arcConfig.API.ProcessorCacheExpiryTime),
	}

	if arcConfig.Prometheus.IsEnabled() {
		handlerStats, err := apiHandler.NewStats()
		if err != nil {
			stopFn()
			return nil, err
		}

		apiOpts = append(apiOpts, apiHandler.WithStats(handlerStats))
	}

	var cachedFinderOpts []func(f *tx_finder.CachedFinder)
	var finderOpts []func(f *tx_finder.Finder)
	var nodeClientOpts []func(client *node_client.NodeClient)
	wocClientOpts := []func(client *woc_client.WocClient){woc_client.WithAuth(arcConfig.API.WocAPIKey)}

	if arcConfig.IsTracingEnabled() {
		cleanup, err := tracing.Enable(logger, "api", arcConfig.Tracing.DialAddr, arcConfig.Tracing.Sample)
		if err != nil {
			logger.Error("failed to enable tracing", slog.String("err", err.Error()))
		} else {
			shutdownFns = append(shutdownFns, cleanup)
		}

		attributes := arcConfig.Tracing.KeyValueAttributes
		hostname, err := os.Hostname()
		if err == nil {
			hostnameAttr := attribute.String("hostname", hostname)
			attributes = append(attributes, hostnameAttr)
		}

		mtmOpts = append(mtmOpts, metamorph.WithClientTracer(attributes...))
		apiOpts = append(apiOpts, apiHandler.WithTracer(attributes...))
		cachedFinderOpts = append(cachedFinderOpts, tx_finder.WithTracerCachedFinder(attributes...))
		finderOpts = append(finderOpts, tx_finder.WithTracerFinder(attributes...))
		nodeClientOpts = append(nodeClientOpts, node_client.WithTracer(attributes...))
		wocClientOpts = append(wocClientOpts, woc_client.WithTracer(attributes...))
	}

	conn, err := grpc_utils.DialGRPC(arcConfig.Metamorph.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, arcConfig.Tracing)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	mtmClient := metamorph.NewClient(
		metamorph_api.NewMetaMorphAPIClient(conn),
		mtmOpts...,
	)

	btcConn, err := grpc_utils.DialGRPC(arcConfig.Blocktx.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, arcConfig.Tracing)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to connect to blocktx server: %v", err)
	}
	blockTxClient := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(btcConn))

	var policy *bitcoin.Settings
	policy, err = getPolicyFromNode(arcConfig.PeerRPC)
	if err != nil {
		policy = arcConfig.API.DefaultPolicy
	}

	wocClient := woc_client.New(arcConfig.API.WocMainnet, wocClientOpts...)

	pc := arcConfig.PeerRPC

	nc, err := node_client.NewRPCClient(pc.User, pc.Port, pc.Host, pc.Password)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to create node client: %v", err)
	}

	nodeClient, err := node_client.New(nc, nodeClientOpts...)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to create node client: %v", err)
	}

	finder := tx_finder.New(mtmClient, nodeClient, wocClient, logger, finderOpts...)
	cachedFinder := tx_finder.NewCached(finder, cachedFinderOpts...)

	defaultAPIHandler, err := apiHandler.NewDefault(logger, mtmClient, blockTxClient, policy, cachedFinder, apiOpts...)
	if err != nil {
		stopFn()
		return nil, err
	}

	// Register the ARC API
	api.RegisterHandlers(echoServer, defaultAPIHandler)

	// Serve HTTP until the world ends.
	go func() {
		logger.Info("Starting API server", slog.String("address", arcConfig.API.Address))
		err := echoServer.Start(arcConfig.API.Address)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				logger.Info("API http server closed")
				return
			}

			logger.Error("Failed to start API server", slog.String("err", err.Error()))
			return
		}
	}()

	return stopFn, nil
}

func setAPIEcho(logger *slog.Logger, cfg *config.APIConfig) *echo.Echo {
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

	// Add event ID to the request context
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			//nolint:staticcheck // use string key on purpose
			reqCtx := context.WithValue(req.Context(), arc_logger.EventIDField, uuid.New().String()) //lint:ignore SA1029 use string key on purpose
			c.SetRequest(req.WithContext(reqCtx))

			return next(c)
		}
	})

	e.Use(otelecho.Middleware("api-server"))

	// Log info about requests
	e.Use(logRequestMiddleware(logger, cfg.RequestExtendedLogs))

	// add prometheus metrics
	e.Use(echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
		Subsystem: "api",
		HistogramOptsFunc: func(opts prometheus.HistogramOpts) prometheus.HistogramOpts {
			if opts.Name == "request_duration_seconds" {
				opts.Buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 30, 60}
			}
			return opts
		},
	}))

	return e
}

func logRequestMiddleware(logger *slog.Logger, extendLog bool) echo.MiddlewareFunc {
	if extendLog {
		return echomiddleware.RequestLoggerWithConfig(extendRequestLogConfig(logger))
	}

	return echomiddleware.RequestLoggerWithConfig(requestLogConfig(logger))
}

func disposeAPI(logger *slog.Logger, echoServer *echo.Echo, mqClient mq.MessageQueueClient, shutdownFns []func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := echoServer.Shutdown(ctx); err != nil {
		logger.Error("Failed to close API echo server", slog.String("err", err.Error()))
	}
	mqClient.Shutdown()

	for _, fn := range shutdownFns {
		fn()
	}
}

func requestLogConfig(logger *slog.Logger) echomiddleware.RequestLoggerConfig {
	return echomiddleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogError:    true,
		HandleError: true, // forwards error to the global error handler, so it can decide appropriate status code
		LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
			ctx := c.Request().Context()

			if v.Error == nil {
				logger.InfoContext(ctx, "REQUEST",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
				)
			} else {
				logger.ErrorContext(ctx, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}
}

func extendRequestLogConfig(logger *slog.Logger) echomiddleware.RequestLoggerConfig {
	return echomiddleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogError:  true,
		LogHeaders: []string{
			"X-CallbackUrl",
			"X-CallbackToken",
			"X-CallbackBatch",
			"X-SkipFeeValidation",
			"X-SkipScriptValidation",
			"X-SkipTxValidation",
			"X-ForceValidation",
			"X-FullStatusUpdates",
			"X-MaxTimeout",
			"X-WaitFor",
			"X-CumulativeFeeValidation",
		},
		HandleError: true, // forwards error to the global error handler, so it can decide appropriate status code
		LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
			ctx := c.Request().Context()

			if v.Error == nil {
				logger.InfoContext(ctx, "REQUEST",
					slog.String("verb", v.Method),
					slog.String("uri", v.URI),
					slog.Any("headers", v.Headers),
					slog.Int("status", v.Status),
				)
			} else {
				logger.ErrorContext(ctx, "REQUEST_ERROR",
					slog.String("verb", v.Method),
					slog.String("uri", v.URI),
					slog.Any("headers", v.Headers),
					slog.Int("status", v.Status),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}
}

func getPolicyFromNode(peerRPCConfig *config.PeerRPCConfig) (*bitcoin.Settings, error) {
	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRPCConfig.User, peerRPCConfig.Password, peerRPCConfig.Host, peerRPCConfig.Port))
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
