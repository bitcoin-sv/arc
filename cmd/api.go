package cmd

import (
	"context"
	"net/http"
	"time"

	"github.com/TAAL-GmbH/arc/api/handler"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	apmecho "github.com/opentracing-contrib/echo"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-utils"

	"github.com/ordishs/gocore"
)

const progname = "arc"

func StartAPIServer(logger utils.Logger) (func(), error) {
	// Set up a basic Echo router
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Recover returns a middleware which recovers from panics anywhere in the chain
	e.Use(echomiddleware.Recover())

	if opentracing.GlobalTracer() != nil {
		e.Use(apmecho.Middleware(progname))
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
	if err := handler.LoadArcHandler(e, logger); err != nil {
		panic(err)
	}

	apiAddress, ok := gocore.Config().Get("arc_httpAddress") //, "localhost:8080")
	if !ok {
		panic("arc_httpAddress not found in config")
	}
	// Serve HTTP until the world ends.
	go func() {
		logger.Infof("Starting API server on %s", apiAddress)
		err := e.Start(apiAddress)
		if err != nil {
			if err == http.ErrServerClosed {
				logger.Infof("API http server closed")
				return
			} else {
				logger.Fatalf("Error starting API server: %v", err)
			}
		}
	}()

	return func() {
		logger.Infof("Shutting down api service")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			logger.Errorf("Error closing API echo server: %v", err)
		}
	}, nil
}
