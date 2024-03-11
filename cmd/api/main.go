package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bitcoin-sv/arc/cmd"
	"github.com/spf13/viper"
)

// Version & commit strings injected at build with -ldflags -X...
var (
	version string
	commit  string
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("failed to run ARC: %v", err)
	}

	os.Exit(0)
}

func run() error {
	viper.SetConfigName("config/config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("failed to read config file config.yaml: %v", err)
	}

	logger, err := cfg.NewLogger()
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}

	logger.Info("starting api", slog.String("version", version), slog.String("commit", commit))

	go func() {
		profilerAddr := viper.GetString("metamorph.profilerAddr")
		if profilerAddr != "" {
			logger.Info(fmt.Sprintf("Starting profiler on http://%s/debug/pprof", profilerAddr))

			err := http.ListenAndServe(profilerAddr, nil)
			if err != nil {
				logger.Error("failed to start profiler server", slog.String("err", err.Error()))
			}
		}
	}()

	shutdown, err := cmd.StartAPIServer(logger)
	if err != nil {
		return fmt.Errorf("failed to start API server: %v", err)
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)

	<-signalChan
	appCleanup(logger, shutdown)

	return nil
}

func appCleanup(logger *slog.Logger, shutdown func()) {
	logger.Info("Shutting down...")
	shutdown()
}
