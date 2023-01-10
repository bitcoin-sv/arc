package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/TAAL-GmbH/arc/cmd"
	"github.com/ordishs/gocore"
)

// Name used by build script for the binaries. (Please keep on single line)
const progname = "arc"

// // Version & commit strings injected at build with -ldflags -X...
var version string
var commit string

var logger = gocore.Log(progname)

func init() {
	gocore.SetInfo(progname, version, commit)
}

func main() {
	stats := gocore.Config().Stats()
	logger.Infof("STATS\n%s\nVERSION\n-------\n%s (%s)\n\n", stats, version, commit)

	go func() {
		profilerAddr, ok := gocore.Config().Get("profilerAddr")
		if ok {
			logger.Infof("Starting profile on http://%s/debug/pprof", profilerAddr)
			logger.Fatalf("%v", http.ListenAndServe(profilerAddr, nil))
		}
	}()

	var startApi bool
	flag.BoolVar(&startApi, "api", false, "start ARC api server")
	flag.BoolVar(&startApi, "a", false, "start ARC api server (shorthand)")

	var startBlockTx bool
	flag.BoolVar(&startBlockTx, "blocktx", false, "start blocktx")
	flag.BoolVar(&startBlockTx, "b", false, "start blocktx (shorthand)")

	var startCallbacker bool
	flag.BoolVar(&startCallbacker, "callbacker", false, "start callbacker")
	flag.BoolVar(&startCallbacker, "c", false, "start callbacker (shorthand)")

	var startMetamorph bool
	flag.BoolVar(&startMetamorph, "metamorph", false, "start metamorph")
	flag.BoolVar(&startMetamorph, "m", false, "start metamorph (shorthand)")

	flag.Parse()

	if !startApi && !startBlockTx && !startCallbacker && !startMetamorph {
		logger.Infof("No service selected, starting all")
		startApi = true
		startBlockTx = true
		startCallbacker = true
		startMetamorph = true
	}

	if startBlockTx {
		logger.Infof("Starting BlockTx")
		var blockTxLogger = gocore.Log("btx")
		go cmd.StartBlockTx(blockTxLogger)
	}

	if startCallbacker {
		logger.Infof("Starting Callbacker")
		var callbackerLogger = gocore.Log("cbk")
		go cmd.StartCallbacker(callbackerLogger)
	}

	if startMetamorph {
		logger.Infof("Starting Metamorph")
		var metamorphLogger = gocore.Log("mtm")
		go cmd.StartMetamorph(metamorphLogger)
	}

	if startApi {
		logger.Infof("Starting ARC api server")
		var apiLogger = gocore.Log("arc")
		go cmd.StartArcAPIServer(apiLogger)
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	<-signalChan
	appCleanup()
	os.Exit(1)
}

func appCleanup() {
	logger.Infof("Shutting down...")
}
