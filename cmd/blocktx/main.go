package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/bitcoin-sv/arc/cmd"
	cfg "github.com/bitcoin-sv/arc/config"

	"github.com/spf13/viper"

	_ "github.com/lib/pq"
)

// // Version & commit strings injected at build with -ldflags -X...
var version string
var commit string

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("failed to read config file config.yaml: %v \n", err)
		return
	}

	logger, err := cfg.NewLogger()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
		return
	}

	logger.Info("starting arc", slog.String("version", version), slog.String("commit", commit))

	go func() {
		profilerAddr := viper.GetString("blocktx.profilerAddr")
		if profilerAddr != "" {
			logger.Info(fmt.Sprintf("Starting profiler on http://%s/debug/pprof", profilerAddr))

			err := http.ListenAndServe(profilerAddr, nil)
			logger.Error("failed to start profiler server", slog.String("err", err.Error()))
		}
	}()

	shutdown, err := cmd.StartBlockTx(logger)
	if err != nil {
		logger.Error("Failed to start blocktx", slog.String("err", err.Error()))
		return
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	<-signalChan

	appCleanup(logger, shutdown)
	os.Exit(1)
}

func appCleanup(logger *slog.Logger, shutdown func()) {
	logger.Info("Shutting down blocktx")
	shutdown()
}
