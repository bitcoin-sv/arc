package main

import (
	"fmt"
	"net/http"

	"github.com/TAAL-GmbH/arc/config"
	"github.com/TAAL-GmbH/arc/dictionary"
	"github.com/TAAL-GmbH/arc/handler"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/mrz1836/go-logger"
)

func main() {

	// Load the Application Configuration
	appConfig, err := config.Load("")
	if err != nil {
		logger.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorLoadingConfig), err.Error())
		return
	}

	// Set up a basic Echo router
	e := echo.New()

	// Add CORS headers to the server - all request origins are allowed
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))

	// use the standard echo logger
	e.Use(echomiddleware.Logger())

	// load the ARC handler from config
	// If you want to customize this for your own server, see examples dir
	if err = handler.LoadArcHandler(e, appConfig); err != nil {
		panic(err)
	}

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", appConfig.Server.IPAddress, appConfig.Server.Port)))
}
