package main

import (
	"os"
	"os/signal"

	"github.com/bitcoin-sv/arc/cmd"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

const progname = "api"

func main() {
	logLevel, _ := gocore.Config().Get("logLevel")
	logger := gocore.Log(progname, gocore.NewLogLevelFromString(logLevel))
	shutdown, err := cmd.StartAPIServer(logger)
	if err != nil {
		logger.Fatalf("Error starting API server: %v", err)
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	<-signalChan

	appCleanup(logger, shutdown)
	os.Exit(1)
}

func appCleanup(logger utils.Logger, shutdown func()) {
	logger.Infof("Shutting down...")
	shutdown()
}
