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
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	cmd "github.com/bitcoin-sv/arc/cmd/arc/services"
	"github.com/bitcoin-sv/arc/config"
	arcLogger "github.com/bitcoin-sv/arc/internal/logger"
	"github.com/bitcoin-sv/arc/internal/version"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("failed to run ARC: %v", err)
	}

	os.Exit(0)
}

func run() error {
	configDir, startAPI, startMetamorph, startBlockTx, startK8sWatcher, startCallbacker, dumpConfigFile := parseFlags()

	arcConfig, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("failed to load app config: %w", err)
	}

	if dumpConfigFile != "" {
		return config.DumpConfig(dumpConfigFile)
	}

	logger, err := arcLogger.NewLogger(arcConfig.LogLevel, arcConfig.LogFormat)
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get host name: %v", err)
	}

	logger = logger.With(slog.String("host", hostname))

	cacheStore, err := cmd.NewCacheStore(arcConfig.Cache)
	if err != nil {
		return fmt.Errorf("failed to create cache store: %v", err)
	}

	logger.Info("Starting ARC", slog.String("version", version.Version), slog.String("commit", version.Commit))

	shutdownFns := make([]func(), 0)

	shutdownCh := make(chan string, 1)

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
		if arcConfig.Prometheus.IsEnabled() {
			logger.Info("Starting prometheus", slog.String("endpoint", arcConfig.Prometheus.Endpoint))
			http.Handle(arcConfig.Prometheus.Endpoint, promhttp.Handler())
			err = http.ListenAndServe(arcConfig.Prometheus.Addr, nil)
			if err != nil {
				logger.Error("failed to start prometheus server", slog.String("err", err.Error()))
			}
		}
	}()

	if !isAnyFlagPassed("api", "blocktx", "metamorph", "k8s-watcher", "callbacker") {
		logger.Info("No service selected, starting all")
		startAPI = true
		startMetamorph = true
		startBlockTx = true
		startCallbacker = true
	}

	if startBlockTx {
		logger.Info("Starting BlockTx")
		shutdown, err := cmd.StartBlockTx(logger, arcConfig, shutdownCh)
		if err != nil {
			return fmt.Errorf("failed to start blocktx: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startMetamorph {
		logger.Info("Starting Metamorph")
		shutdown, err := cmd.StartMetamorph(logger, arcConfig, cacheStore, shutdownCh)
		if err != nil {
			return fmt.Errorf("failed to start metamorph: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startAPI {
		logger.Info("Starting API")
		shutdown, err := cmd.StartAPIServer(logger, arcConfig, shutdownCh)
		if err != nil {
			return fmt.Errorf("failed to start api: %v", err)
		}

		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startK8sWatcher {
		logger.Info("Starting K8s-Watcher")
		shutdown, err := cmd.StartK8sWatcher(logger, arcConfig)
		if err != nil {
			return fmt.Errorf("failed to start k8s-watcher: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startCallbacker {
		shutdown, err := cmd.StartCallbacker(logger, arcConfig, shutdownCh)
		if err != nil {
			return fmt.Errorf("failed to start callbacker: %v", err)
		}
		shutdownFns = append(shutdownFns, shutdown)
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	select {
	case reason := <-shutdownCh:
		logger.Info("Received shutdown signal", slog.String("reason", reason))
	case sig := <-signalChan:
		logger.Info("Received shutdown signal", slog.String("reason", sig.String()))
	}
	appCleanup(logger, shutdownFns)

	return nil
}

func appCleanup(logger *slog.Logger, shutdownFns []func()) {
	logger.Info("cleaning up")
	for _, fn := range shutdownFns {
		fn()
	}
}

func parseFlags() (string, bool, bool, bool, bool, bool, string) {
	startAPI := flag.Bool("api", false, "start ARC api server")
	startMetamorph := flag.Bool("metamorph", false, "start metamorph")
	startBlockTx := flag.Bool("blocktx", false, "start blocktx")
	startK8sWatcher := flag.Bool("k8s-watcher", false, "start k8s-watcher")
	startCallbacker := flag.Bool("callbacker", false, "start callbacker")
	help := flag.Bool("help", false, "Show help")
	dumpConfigFile := flag.String("dump_config", "", "dump config to specified file and exit")
	configDir := flag.String("config", "", "path to configuration file")

	flag.Parse()

	if *help {
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
		fmt.Println("    -callbacker=<true|false>")
		fmt.Println("          whether to start callbacker (default=true)")
		fmt.Println("")
		fmt.Println("    -config=/location")
		fmt.Println("          directory to look for config (default='')")
		fmt.Println("")
		fmt.Println("    -dump_config=/file.yaml")
		fmt.Println("          dump config to specified file and exit (default='config/dumped_config.yaml')")
		fmt.Println("")
		os.Exit(0)
	}

	return *configDir, *startAPI, *startMetamorph, *startBlockTx, *startK8sWatcher, *startCallbacker, *dumpConfigFile
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
