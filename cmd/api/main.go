package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bitcoin-sv/arc/cmd"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"github.com/spf13/viper"
)

const progname = "api"

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("failed to read config file config.yaml: %v \n", err)
		return
	}

	logLevel := viper.GetString("logLevel")
	logger := gocore.Log(progname, gocore.NewLogLevelFromString(logLevel))
	shutdown, err := cmd.StartAPIServer(logger)
	if err != nil {
		logger.Fatalf("Error starting API server: %v", err)
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)

	<-signalChan
	appCleanup(logger, shutdown)
	os.Exit(0)
}

func appCleanup(logger utils.Logger, shutdown func()) {
	logger.Infof("Shutting down...")
	shutdown()
}
