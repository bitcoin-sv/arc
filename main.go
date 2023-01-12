package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/TAAL-GmbH/arc/cmd"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
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
	logger := gocore.Log(progname)

	stats := gocore.Config().Stats()
	logger.Infof("STATS\n%s\nVERSION\n-------\n%s (%s)\n\n", stats, version, commit)

	go func() {
		profilerAddr, ok := gocore.Config().Get("profilerAddr")
		if ok {
			logger.Infof("Starting profile on http://%s/debug/pprof", profilerAddr)
			logger.Fatalf("%v", http.ListenAndServe(profilerAddr, nil))
		}
	}()

	startApi := flag.Bool("api", false, "start ARC api server")
	startBlockTx := flag.Bool("blocktx", false, "start blocktx")
	startCallbacker := flag.Bool("callbacker", false, "start callbacker")
	startMetamorph := flag.Bool("metamorph", false, "start metamorph")
	useTracer := flag.Bool("tracer", false, "start tracer")

	flag.Parse()

	if useTracer != nil && *useTracer {
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
		*startBlockTx = true
		*startCallbacker = true
		*startMetamorph = true
	}

	if startBlockTx != nil && *startBlockTx {
		logger.Infof("Starting BlockTx")
		var blockTxLogger = gocore.Log("btx")
		go cmd.StartBlockTx(blockTxLogger)
	}

	if startCallbacker != nil && *startCallbacker {
		logger.Infof("Starting Callbacker")
		var callbackerLogger = gocore.Log("cbk")
		go cmd.StartCallbacker(callbackerLogger)
	}

	if startMetamorph != nil && *startMetamorph {
		logger.Infof("Starting Metamorph")
		var metamorphLogger = gocore.Log("mtm")
		go cmd.StartMetamorph(metamorphLogger)
	}

	if startApi != nil && *startApi {
		logger.Infof("Starting ARC api server")
		var apiLogger = gocore.Log("api")
		go cmd.StartAPIServer(apiLogger)
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	<-signalChan
	appCleanup(logger)
	os.Exit(1)
}

func appCleanup(logger utils.Logger) {
	logger.Infof("Shutting down...")
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
