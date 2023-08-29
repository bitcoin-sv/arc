package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"

	"github.com/bitcoin-sv/arc/cmd"
	"github.com/bitcoin-sv/arc/tracing"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-utils"
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
		return
	}

	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")      // optionally look for config in the working directory
	err := viper.ReadInConfig()   // Find and read the config file
	if err != nil {               // Handle errors reading the config file
		fmt.Printf("failed to read config file config.yaml: %v \n", err)
		return
	}

	logLevel := viper.GetString("logLevel")
	logger := gocore.Log(progname, gocore.NewLogLevelFromString(logLevel))

	logger.Infof("VERSION\n-------\n%s (%s)\n\n", version, commit)

	go func() {
		profilerAddr := viper.GetString("profilerAddr")
		if profilerAddr != "" {
			logger.Infof("Starting profile on http://%s/debug/pprof", profilerAddr)
			logger.Fatalf("%v", http.ListenAndServe(profilerAddr, nil))
		}
	}()

	prometheusEndpoint := viper.GetString("prometheusEndpoint")
	if prometheusEndpoint != "" {
		logger.Infof("Starting prometheus endpoint on %s", prometheusEndpoint)
		http.Handle(prometheusEndpoint, promhttp.Handler())
	}

	tracingOn := viper.GetBool("tracing")
	if (useTracer != nil && *useTracer) || tracingOn {
		logger.Infof("Starting tracer")
		// Start the tracer
		tracer, closer := tracing.InitTracer(logger, progname)
		defer closer.Close()

		if tracer != nil {
			// set the global tracer to use in all services
			opentracing.SetGlobalTracer(tracer)
		}
	}

	if !isFlagPassed("api") && !isFlagPassed("blocktx") && !isFlagPassed("callbacker") && !isFlagPassed("metamorph") {
		logger.Infof("No service selected, starting all")
		*startApi = true
		*startMetamorph = true
		*startBlockTx = true
		*startCallbacker = true
	}

	// Check the settings to see it the service has a listen address

	if v := viper.GetString("arc_httpAddress"); v == "" {
		*startApi = false
	}
	if v := viper.GetString("metamorph_grpcAddress"); v == "" {
		*startMetamorph = false
	}
	if v := viper.GetString("blocktx_grpcAddress"); v == "" {
		*startBlockTx = false
	}
	if v := viper.GetString("callbacker_grpcAddress"); v == "" {
		*startCallbacker = false
	}

	shutdownFns := make([]func(), 0)

	if startBlockTx != nil && *startBlockTx {
		logger.Infof("Starting BlockTx")
		var blockTxLogger = gocore.Log("btx", gocore.NewLogLevelFromString(logLevel))
		if blockTxShutdown, err := cmd.StartBlockTx(blockTxLogger); err != nil {
			logger.Fatalf("Error starting blocktx: %v", err)
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
		logger.Infof("Starting Callbacker")
		var callbackerLogger = gocore.Log("cbk", gocore.NewLogLevelFromString(logLevel))
		if callbackerShutdown, err := cmd.StartCallbacker(callbackerLogger); err != nil {
			logger.Fatalf("Error starting callbacker: %v", err)
		} else {
			shutdownFns = append(shutdownFns, func() {
				callbackerShutdown()
			})
		}
	}

	if startMetamorph != nil && *startMetamorph {
		logger.Infof("Starting Metamorph")
		var metamorphLogger = gocore.Log("mtm", gocore.NewLogLevelFromString(logLevel))

		if metamorphShutdown, err := cmd.StartMetamorph(metamorphLogger); err != nil {
			logger.Fatalf("Error starting metamorph: %v", err)
		} else {
			shutdownFns = append(shutdownFns, func() {
				metamorphShutdown()
			})
		}
	}

	if startApi != nil && *startApi {
		var apiLogger = gocore.Log("api", gocore.NewLogLevelFromString(logLevel))
		if apiShutdown, err := cmd.StartAPIServer(apiLogger); err != nil {
			logger.Fatalf("Error starting api server: %v", err)
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

func appCleanup(logger utils.Logger, shutdownFns []func()) {
	logger.Infof("Shutting down...")

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
