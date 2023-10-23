package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/bitcoin-sv/arc/cmd"
	cfg "github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/tracing"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/gocore"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

// Name used by build script for the binaries. (Please keep on single line)
const progname = "arc"

// // Version & commit strings injected at build with -ldflags -X...
var version string
var commit string

func init() {
	gocore.SetInfo(progname, version, commit)
}

func main() {
	startApi := flag.Bool("api", false, "start ARC api server")
	startMetamorph := flag.Bool("metamorph", false, "start metamorph")
	startBlockTx := flag.Bool("blocktx", false, "start blocktx")
	startCallbacker := flag.Bool("callbacker", false, "start callbacker")
	useTracer := flag.Bool("tracer", false, "start tracer")
	help := flag.Bool("help", false, "Show help")
	config := flag.String("config", ".", "path to configuration yaml file")

	flag.Parse()

	if help != nil && *help {
		fmt.Println("usage: main [options]")
		fmt.Println("where options are:")
		fmt.Println("")
		fmt.Println("    -api=<true|false>")
		fmt.Println("          whether to start ARC api server (default=true)")
		fmt.Println("")
		fmt.Println("    -metamorph=<true|false>")
		fmt.Println("          whether to start metamorph (default=true)")
		fmt.Println("")
		fmt.Println("    -blocktx=<true|false>")
		fmt.Println("          whether to start block tx (default=true)")
		fmt.Println("")
		fmt.Println("    -callbacker=<true|false>")
		fmt.Println("          whether to start callbacker (default=true)")
		fmt.Println("")
		fmt.Println("    -tracer=<true|false>")
		fmt.Println("          whether to start the Jaeger tracer (default=false)")
		fmt.Println("")
		fmt.Println("    -config=/location")
		fmt.Println("          directory to look for config.yaml (default='')")
		fmt.Println("")
		return
	}

	viper.AddConfigPath(*config) // optionally look for config in the working directory
	viper.AutomaticEnv()         // read in environment variables that match
	viper.SetEnvPrefix("ARC")
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		log.Fatalf("failed to read config file: %v \n", err)
		return
	}

	logLevel := viper.GetString("logLevel")
	logger, err := cfg.NewLogger()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
		return
	}

	logger.Info("starting arc", slog.String("version", version), slog.String("commit", commit))

	go func() {
		profilerAddr := viper.GetString("profilerAddr")
		if profilerAddr != "" {
			logger.Info(fmt.Sprintf("Starting profiler on http://%s/debug/pprof", profilerAddr))

			err := http.ListenAndServe(profilerAddr, nil)
			logger.Error("failed to start profiler server", slog.String("err", err.Error()))
		}
	}()

	prometheusEndpoint := viper.GetString("prometheusEndpoint")
	if prometheusEndpoint != "" {
		logger.Info("Starting prometheus", slog.String("endpoint", prometheusEndpoint))
		http.Handle(prometheusEndpoint, promhttp.Handler())
	}

	tracingOn := viper.GetBool("tracing")
	if (useTracer != nil && *useTracer) || tracingOn {
		logger.Info("Starting tracer")
		// Start the tracer
		tracer, closer, err := tracing.InitTracer(progname)
		if err != nil {
			logger.Error("failed to initialise tracer", slog.String("err", err.Error()))
		}

		defer closer.Close()

		if tracer != nil {
			// set the global tracer to use in all services
			opentracing.SetGlobalTracer(tracer)
		}
	}

	if !isFlagPassed("api") && !isFlagPassed("blocktx") && !isFlagPassed("callbacker") && !isFlagPassed("metamorph") {
		logger.Info("No service selected, starting all")
		*startApi = true
		*startMetamorph = true
		*startBlockTx = true
		*startCallbacker = true
	}

	// Check the settings to see it the service has a listen address

	if v := viper.GetString("api.address"); v == "" {
		*startApi = false
	}
	if v := viper.GetString("metamorph.listenAddr"); v == "" {
		*startMetamorph = false
	}
	if v := viper.GetString("blocktx.listenAddr"); v == "" {
		*startBlockTx = false
	}
	if v := viper.GetString("callbacker.listenAddr"); v == "" {
		*startCallbacker = false
	}

	shutdownFns := make([]func(), 0)

	if startBlockTx != nil && *startBlockTx {
		logger.Info("Starting BlockTx")

		var blockTxLogger = gocore.Log("btx", gocore.NewLogLevelFromString(logLevel))
		if blockTxShutdown, err := cmd.StartBlockTx(blockTxLogger); err != nil {
			logger.Error("Failed to start blocktx", slog.String("err", err.Error()))
		} else {
			shutdownFns = append(shutdownFns, func() {
				blockTxShutdown()
			})
		}
	}

	statisticsServerAddr := viper.GetString("statisticsServerAddress")
	if statisticsServerAddr != "" {
		go func() {
			gocore.StartStatsServer(statisticsServerAddr)
		}()
	}

	if startCallbacker != nil && *startCallbacker {
		logger.Info("Starting Callbacker")
		callbackerLogger, err := cfg.NewLogger()
		if err != nil {
			logger.Error("Failed to get logger for callbacker", slog.String("err", err.Error()))
		}

		if callbackerShutdown, err := cmd.StartCallbacker(callbackerLogger.With(slog.String("service", "clb"))); err != nil {
			logger.Error("Failed to start callbacker", slog.String("err", err.Error()))
		} else {
			shutdownFns = append(shutdownFns, func() {
				callbackerShutdown()
			})
		}
	}

	if startMetamorph != nil && *startMetamorph {
		logger.Info("Starting Metamorph")

		if metamorphShutdown, err := cmd.StartMetamorph(logger); err != nil {
			logger.Error("Error starting metamorph", slog.String("err", err.Error()))
		} else {
			shutdownFns = append(shutdownFns, func() {
				metamorphShutdown()
			})
		}
	}

	if startApi != nil && *startApi {
		var apiLogger = gocore.Log("api", gocore.NewLogLevelFromString(logLevel))
		if apiShutdown, err := cmd.StartAPIServer(apiLogger); err != nil {
			logger.Error("Error starting api server", slog.String("err", err.Error()))
		} else {
			shutdownFns = append(shutdownFns, func() {
				apiShutdown()
			})
		}
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	<-signalChan
	appCleanup(logger, shutdownFns)
	os.Exit(1)
}

func appCleanup(logger *slog.Logger, shutdownFns []func()) {
	logger.Info("Shutting down")

	var wg sync.WaitGroup
	for _, fn := range shutdownFns {
		// fire the shutdown functions off in the background
		// they might be relying on each other, and this allows them to gracefully stop
		wg.Add(1)
		go func(fn func()) {
			defer wg.Done()
			fn()
		}(fn)
	}
	wg.Wait()
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
