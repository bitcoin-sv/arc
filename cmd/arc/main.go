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
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bitcoin-sv/arc/cmd/services"
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
	configFiles, startAPI, startMetamorph, startBlockTx, startK8sWatcher, startCallbacker, dumpConfigFile := parseFlags()
	configFileSlice := strings.Split(configFiles, ",")

	arcConfig, err := config.Load(configFileSlice...)
	if err != nil {
		return fmt.Errorf("failed to load app config: %w", err)
	}

	if dumpConfigFile != "" {
		return config.DumpConfig(dumpConfigFile)
	}

	logger, err := arcLogger.NewLogger(arcConfig.Common.LogLevel, arcConfig.Common.LogFormat)
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get host name: %v", err)
	}

	logger = logger.With(slog.String("host", hostname))
	shutdownFns, err := startServices(arcConfig, logger, startAPI, startMetamorph, startBlockTx, startK8sWatcher, startCallbacker)
	if err != nil {
		return err
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	<-signalChan

	logger.Info("Received signal to shutdown")

	appCleanup(logger, shutdownFns)

	return nil
}

func startServices(arcConfig *config.ArcConfig, logger *slog.Logger, startAPI bool, startMetamorph bool, startBlockTx bool, startK8sWatcher bool, startCallbacker bool) ([]func(), error) {
	logger.Info("Starting ARC", slog.String("version", version.Version), slog.String("commit", version.Commit))
	shutdownFns := make([]func(), 0)

	go func() {
		if arcConfig.Common.ProfilerAddr != "" {
			logger.Info(fmt.Sprintf("Starting profiler on http://%s/debug/pprof", arcConfig.Common.ProfilerAddr))

			err := http.ListenAndServe(arcConfig.Common.ProfilerAddr, nil)
			if err != nil {
				logger.Error("failed to start profiler server", slog.String("err", err.Error()))
			}
		}
	}()

	go func() {
		if arcConfig.Common.Prometheus.IsEnabled() {
			logger.Info("Starting prometheus", slog.String("endpoint", arcConfig.Common.Prometheus.Endpoint))
			http.Handle(arcConfig.Common.Prometheus.Endpoint, promhttp.Handler())
			err := http.ListenAndServe(arcConfig.Common.Prometheus.Addr, nil)
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
		shutdown, err := services.StartBlockTx(logger, arcConfig.Blocktx, arcConfig.Common)
		if err != nil {
			return nil, fmt.Errorf("failed to start blocktx: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startMetamorph {
		logger.Info("Starting Metamorph")
		shutdown, err := services.StartMetamorph(logger, arcConfig.Metamorph, arcConfig.Common)
		if err != nil {
			return nil, fmt.Errorf("failed to start metamorph: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startAPI {
		logger.Info("Starting API")
		shutdown, err := services.StartAPIServer(logger, arcConfig.API, arcConfig.Common)
		if err != nil {
			return nil, fmt.Errorf("failed to start api: %v", err)
		}

		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startK8sWatcher {
		logger.Info("Starting K8s-Watcher")
		shutdown, err := services.StartK8sWatcher(logger, arcConfig.K8sWatcher, arcConfig.Common)
		if err != nil {
			return nil, fmt.Errorf("failed to start k8s-watcher: %v", err)
		}
		shutdownFns = append(shutdownFns, func() { shutdown() })
	}

	if startCallbacker {
		shutdown, err := services.StartCallbacker(logger, arcConfig.Callbacker, arcConfig.Common)
		if err != nil {
			return nil, fmt.Errorf("failed to start callbacker: %v", err)
		}
		shutdownFns = append(shutdownFns, shutdown)
	}
	return shutdownFns, nil
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
	configFiles := flag.String("config", "", "comma separated list of paths to configuration files e.g. -config=/path/to/config1.yaml,/path/to/config2.yaml")

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
		fmt.Println("    -config=/path/to/config.yaml")
		fmt.Println("          comma separated path to config file (default='./config_common.yaml,./config_api')")
		fmt.Println("")
		fmt.Println("    -dump_config=/file.yaml")
		fmt.Println("          dump config to specified file and exit (default='config/dumped_config.yaml')")
		fmt.Println("")
		os.Exit(0)
	}

	return *configFiles, *startAPI, *startMetamorph, *startBlockTx, *startK8sWatcher, *startCallbacker, *dumpConfigFile
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
