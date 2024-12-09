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
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	arc_logger "github.com/bitcoin-sv/arc/internal/logger"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/node_client"
	"github.com/bitcoin-sv/arc/internal/tracing"
	tx_finder "github.com/bitcoin-sv/arc/internal/tx_finder"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/api/handler"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
)

func StartAPIServer(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	logger = logger.With(slog.String("service", "api"))

	e := setAPIEcho(logger, arcConfig)

	// load the ARC handler from config
	// If you want to customize this for your own server, see examples dir
	// check the swagger definition against our requests
	handler.CheckSwagger(e)

	mqClient, err := natsMqClient(logger, arcConfig)
	if err != nil {
		return nil, err
	}

	mtmOpts := []func(*metamorph.Metamorph){
		metamorph.WithMqClient(mqClient),
		metamorph.WithLogger(logger),
	}
	// TODO: WithSecurityConfig(appConfig.Security)
	apiOpts := []handler.Option{
		handler.WithCallbackURLRestrictions(arcConfig.Metamorph.RejectCallbackContaining),
	}
	var cachedFinderOpts []func(f *tx_finder.CachedFinder)
	var finderOpts []func(f *tx_finder.Finder)
	var nodeClientOpts []func(client *node_client.NodeClient)

	shutdownFns := make([]func(), 0)

	if arcConfig.IsTracingEnabled() {
		cleanup, err := tracing.Enable(logger, "api", arcConfig.Tracing)
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

		mtmOpts = append(mtmOpts, metamorph.WithTracer(attributes...))
		apiOpts = append(apiOpts, handler.WithTracer(attributes...))
		cachedFinderOpts = append(cachedFinderOpts, tx_finder.WithTracerCachedFinder(attributes...))
		finderOpts = append(finderOpts, tx_finder.WithTracerFinder(attributes...))
		nodeClientOpts = append(nodeClientOpts, node_client.WithTracer(attributes...))
	}

	conn, err := metamorph.DialGRPC(arcConfig.Metamorph.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, arcConfig.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	metamorphClient := metamorph.NewClient(
		metamorph_api.NewMetaMorphAPIClient(conn),
		mtmOpts...,
	)

	btcConn, err := blocktx.DialGRPC(arcConfig.Blocktx.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, arcConfig.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to blocktx server: %v", err)
	}
	blockTxClient := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(btcConn))

	var policy *bitcoin.Settings
	policy, err = getPolicyFromNode(arcConfig.PeerRPC)
	if err != nil {
		policy = arcConfig.API.DefaultPolicy
	}

	wocClient := woc_client.New(arcConfig.API.WocMainnet, woc_client.WithAuth(arcConfig.API.WocAPIKey))

	pc := arcConfig.PeerRPC
	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", pc.User, pc.Password, pc.Host, pc.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to parse node rpc url: %w", err)
	}

	bitcoinClient, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create bitcoin client: %w", err)
	}

	nodeClient, err := node_client.New(bitcoinClient, nodeClientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create node client: %v", err)
	}

	finder := tx_finder.New(metamorphClient, nodeClient, wocClient, logger, finderOpts...)
	cachedFinder := tx_finder.NewCached(finder, cachedFinderOpts...)

	apiHandler, err := handler.NewDefault(logger, metamorphClient, blockTxClient, policy, cachedFinder, apiOpts...)
	if err != nil {
		return nil, err
	}

	// Register the ARC API
	api.RegisterHandlers(e, apiHandler)

	// Serve HTTP until the world ends.
	go func() {
		logger.Info("Starting API server", slog.String("address", arcConfig.API.Address))
		err := e.Start(arcConfig.API.Address)
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

		for _, fn := range shutdownFns {
			fn()
		}
	}, nil
}

func setAPIEcho(logger *slog.Logger, arcConfig *config.ArcConfig) *echo.Echo {
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
	e.Use(logRequestMiddleware(logger, arcConfig.API.RequestExtendedLogs))

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

func natsMqClient(logger *slog.Logger, arcConfig *config.ArcConfig) (metamorph.MessageQueueClient, error) {
	logger = logger.With("module", "nats")

	conn, err := nats_connection.New(arcConfig.MessageQueue.URL, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
	}

	var mqClient metamorph.MessageQueueClient
	if arcConfig.MessageQueue.Streaming.Enabled {
		var opts []nats_jetstream.Option
		if arcConfig.MessageQueue.Streaming.FileStorage {
			opts = append(opts, nats_jetstream.WithFileStorage())
		}

		if arcConfig.Tracing.Enabled {
			opts = append(opts, nats_jetstream.WithTracer(arcConfig.Tracing.KeyValueAttributes...))
		}

		mqClient, err = nats_jetstream.New(conn, logger, []string{metamorph.SubmitTxTopic}, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create nats client: %v", err)
		}
	} else {
		opts := []nats_core.Option{nats_core.WithLogger(logger)}
		if arcConfig.Tracing.Enabled {
			opts = append(opts, nats_core.WithTracer(arcConfig.Tracing.KeyValueAttributes...))
		}
		mqClient = nats_core.New(conn, opts...)
	}

	return mqClient, nil
}

func logRequestMiddleware(logger *slog.Logger, extendLog bool) echo.MiddlewareFunc {
	if extendLog {
		return echomiddleware.RequestLoggerWithConfig(extendRequestLogConfig(logger))
	}

	return echomiddleware.RequestLoggerWithConfig(requestLogConfig(logger))
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
