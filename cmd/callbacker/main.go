package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/bitcoin-sv/arc/cmd"
	"github.com/bitcoin-sv/arc/config"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

// Version & commit strings injected at build with -ldflags -X...
var version string
var commit string

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("failed to read config file config.yaml: %v", err)
		return
	}

	logger, err := config.NewLogger()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
		return
	}

	logger.Info("starting callbacker", slog.String("version", version), slog.String("commit", commit))

	go func() {
		profilerAddr := viper.GetString("callbacker.profilerAddr")
		if profilerAddr != "" {
			logger.Info(fmt.Sprintf("Starting profiler on http://%s/debug/pprof", profilerAddr))
			err := http.ListenAndServe(profilerAddr, nil)
			logger.Error("failed to start profiler server", slog.String("err", err.Error()))
			return
		}
	}()

	shutdown, err := cmd.StartCallbacker(logger)
	if err != nil {
		logger.Error("Failed to start callbacker", slog.String("err", err.Error()))
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)

	<-signalChan
	appCleanup(logger, shutdown)
	os.Exit(0)
}

func appCleanup(logger *slog.Logger, shutdown func()) {
	logger.Info("Shutting down")
	shutdown()
}
