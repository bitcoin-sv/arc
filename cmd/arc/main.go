package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	cmd "github.com/bitcoin-sv/arc/cmd/arc/services"
	cfg "github.com/bitcoin-sv/arc/internal/config"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("failed to run ARC: %v", err)
	}

	os.Exit(0)
}

func run() error {
	startApi := flag.Bool("api", false, "start ARC api server")
	startMetamorph := flag.Bool("metamorph", false, "start metamorph")
	startBlockTx := flag.Bool("blocktx", false, "start blocktx")
	startK8sWatcher := flag.Bool("k8s-watcher", false, "start k8s-watcher")
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
		fmt.Println("    -k8s-watcher=<true|false>")
		fmt.Println("          whether to start k8s-watcher (default=true)")
		fmt.Println("")
		fmt.Println("    -config=/location")
		fmt.Println("          directory to look for config.yaml (default='')")
		fmt.Println("")
		return nil
	}

	viper.AddConfigPath(*config) // optionally look for config in the working directory
	viper.AutomaticEnv()         // read in environment variables that match
	viper.SetEnvPrefix("ARC")
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		return fmt.Errorf("failed to read config file: %v", err)
	}

	logger, err := cfg.NewLogger()
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get host name: %v", err)
	}

	logger = logger.With(slog.String("host", hostname))

	logger.Info("Starting arc", slog.String("version", version.Version), slog.String("commit", version.Commit))

	shutdownFns := make([]func(), 0)

	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered from panic", "panic", r, slog.String("stacktrace", string(debug.Stack())))
		}
	}()

	tracingAddr := viper.GetString("tracing.dialAddr")
	tracingEnabled := false
	if tracingAddr != "" {
		ctx := context.Background()

		exporter, err := tracing.NewExporter(ctx, tracingAddr)
		if err != nil {
			return fmt.Errorf("failed to initialize exporter: %v", err)
		}

		tp, err := tracing.NewTraceProvider(exporter, "arc")
		if err != nil {
			return fmt.Errorf("failed to create trace provider: %v", err)
		}

		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
		tracingEnabled = true

		cleanup := func() {
			err = exporter.Shutdown(ctx)
			if err != nil {
				logger.Error("Failed to shutdown exporter", slog.String("err", err.Error()))
			}

			err = tp.Shutdown(ctx)
			if err != nil {
				logger.Error("Failed to shutdown tracing provider", slog.String("err", err.Error()))
			}
		}
		shutdownFns = append(shutdownFns, cleanup)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Recovered from panic", "panic", r, slog.String("stacktrace", string(debug.Stack())))
			}
		}()
		profilerAddr := viper.GetString("profilerAddr")
		if profilerAddr != "" {
			logger.Info(fmt.Sprintf("Starting profiler on http://%s/debug/pprof", profilerAddr))

			err := http.ListenAndServe(profilerAddr, nil)
			if err != nil {
				logger.Error("failed to start profiler server", slog.String("err", err.Error()))
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Recovered from panic", "panic", r, slog.String("stacktrace", string(debug.Stack())))
			}
		}()

		prometheusAddr := viper.GetString("prometheusAddr")
		prometheusEndpoint := viper.GetString("prometheusEndpoint")
		if prometheusEndpoint != "" && prometheusAddr != "" {
			logger.Info("Starting prometheus", slog.String("endpoint", prometheusEndpoint))
			http.Handle(prometheusEndpoint, promhttp.Handler())
			err = http.ListenAndServe(prometheusAddr, nil)
			if err != nil {
				logger.Error("failed to start prometheus server", slog.String("err", err.Error()))
			}
		}
	}()

	if !isFlagPassed("api") &&
		!isFlagPassed("blocktx") &&
		!isFlagPassed("callbacker") &&
		!isFlagPassed("metamorph") &&
		!isFlagPassed("k8s-watcher") &&
		!isFlagPassed("background-worker") {
		logger.Info("No service selected, starting all")
		*startApi = true
		*startMetamorph = true
		*startBlockTx = true
	}

	if startBlockTx != nil && *startBlockTx {
		logger.Info("Starting BlockTx")
		shutdown, err := cmd.StartBlockTx(logger, tracingEnabled)
		if err != nil {
			return fmt.Errorf("failed to start blocktx: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startMetamorph != nil && *startMetamorph {
		logger.Info("Starting Metamorph")
		shutdown, err := cmd.StartMetamorph(logger)
		if err != nil {
			return fmt.Errorf("failed to start metamorph: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startApi != nil && *startApi {
		logger.Info("Starting API")
		shutdown, err := cmd.StartAPIServer(logger)
		if err != nil {
			return fmt.Errorf("failed to start api: %v", err)
		}

		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startK8sWatcher != nil && *startK8sWatcher {
		logger.Info("Starting K8s-Watcher")
		shutdown, err := cmd.StartK8sWatcher(logger)
		if err != nil {
			return fmt.Errorf("failed to start k8s-watcher: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	<-signalChan
	for _, fn := range shutdownFns {
		fn()
	}
	return nil
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
