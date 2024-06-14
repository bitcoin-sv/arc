package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	cmd "github.com/bitcoin-sv/arc/cmd/arc/services"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/logger"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	configFile := flag.String("config", ".", "path to configuration yaml file")

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

	arcConfig, err := config.Load(*configFile)
	if err != nil {
		return fmt.Errorf("failed to load app config: %w", err)
	}

	logger, err := logger.NewLogger(arcConfig.LogLevel, arcConfig.LogFormat)
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

	if arcConfig.Tracing != nil {
		cleanup, err := enableTracing(logger, arcConfig.Tracing.DialAddr)
		if err != nil {
			return err
		}
		shutdownFns = append(shutdownFns, cleanup)
	}

	go func() {
		if arcConfig.ProfilerAddr != "" {
			logger.Info(fmt.Sprintf("Starting profiler on http://%s/debug/pprof", arcConfig.ProfilerAddr))

			err := http.ListenAndServe(arcConfig.ProfilerAddr, nil)
			if err != nil {
				logger.Error("failed to start profiler server", slog.String("err", err.Error()))
			}
		}
	}()

	go func() {
		if arcConfig.PrometheusEndpoint != "" && arcConfig.ProfilerAddr != "" {
			logger.Info("Starting prometheus", slog.String("endpoint", arcConfig.PrometheusEndpoint))
			http.Handle(arcConfig.PrometheusEndpoint, promhttp.Handler())
			err = http.ListenAndServe(arcConfig.ProfilerAddr, nil)
			if err != nil {
				logger.Error("failed to start prometheus server", slog.String("err", err.Error()))
			}
		}
	}()

	if !isAnyFlagPassed("api", "blocktx", "metamorph", "k8s-watcher") {
		logger.Info("No service selected, starting all")
		*startApi = true
		*startMetamorph = true
		*startBlockTx = true
	}

	if *startBlockTx {
		logger.Info("Starting BlockTx")
		shutdown, err := cmd.StartBlockTx(logger, arcConfig)
		if err != nil {
			return fmt.Errorf("failed to start blocktx: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if *startMetamorph {
		logger.Info("Starting Metamorph")
		shutdown, err := cmd.StartMetamorph(logger)
		if err != nil {
			return fmt.Errorf("failed to start metamorph: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if *startApi {
		logger.Info("Starting API")
		shutdown, err := cmd.StartAPIServer(logger, arcConfig)
		if err != nil {
			return fmt.Errorf("failed to start api: %v", err)
		}

		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if *startK8sWatcher {
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
	appCleanup(logger, shutdownFns)

	return nil
}

func appCleanup(logger *slog.Logger, shutdownFns []func()) {
	logger.Info("Shutting down")
	for _, fn := range shutdownFns {
		fn()
	}
}

func isAnyFlagPassed(flags ...string) bool {
	for _, name := range flags {
		found := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == name {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func enableTracing(logger *slog.Logger, tracingAddr string) (func(), error) {
	if tracingAddr == "" {
		return nil, errors.New("tracing enabled, but tracing addressj empty")
	}

	ctx := context.Background()

	exporter, err := tracing.NewExporter(ctx, tracingAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize exporter: %v", err)
	}

	tp, err := tracing.NewTraceProvider(exporter, "arc")
	if err != nil {
		return nil, fmt.Errorf("failed to create trace provider: %v", err)
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

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

	return cleanup, nil
}
