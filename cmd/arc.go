package cmd

import (
	"net/http"

	"github.com/TAAL-GmbH/arc/api/handler"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/ordishs/gocore"
)

func StartArcAPIServer(logger *gocore.Logger) {
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
	if err := handler.LoadArcHandler(e, logger); err != nil {
		panic(err)
	}

	apiAddress, ok := gocore.Config().Get("arc_httpAddress") //, "localhost:8080")
	if !ok {
		panic("arc_httpAddress not found in config")
	}
	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(apiAddress))
}
