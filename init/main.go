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
	"syscall"

	cmd "github.com/bitcoin-sv/arc/cmd/starters"
	cfg "github.com/bitcoin-sv/arc/internal/helpers"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

// Name used by build script for the binaries. (Please keep on single line)
const progname = "arc"

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
	startBackgroundWorker := flag.Bool("background-worker", false, "start background-worker")
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
		fmt.Println("    -k8s-watcher=<true|false>")
		fmt.Println("          whether to start k8s-watcher (default=true)")
		fmt.Println("")
		fmt.Println("    -background-worker=<true|false>")
		fmt.Println("          whether to background-worker (default=true)")
		fmt.Println("")
		fmt.Println("    -tracer=<true|false>")
		fmt.Println("          whether to start the Jaeger tracer (default=false)")
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

	go func() {
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

	shutdownFns := make([]func(), 0)

	if startBlockTx != nil && *startBlockTx {
		logger.Info("Starting BlockTx")
		shutdown, err := cmd.StartBlockTx(logger)
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

	if startBackgroundWorker != nil && *startBackgroundWorker {
		logger.Info("Starting Background-Worker")
		shutdown, err := cmd.StartBackGroundWorker(logger)
		if err != nil {
			return fmt.Errorf("failed to start background-worker: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)

	<-signalChan
	appCleanup(logger, shutdownFns)

	return nil
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
