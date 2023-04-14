package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/bitcoin-sv/arc/cmd"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"

	_ "github.com/lib/pq"
)

// Name used by build script for the binaries. (Please keep on single line)
const progname = "block-tx"

// // Version & commit strings injected at build with -ldflags -X...
var version string
var commit string

func init() {
	gocore.SetInfo(progname, version, commit)
}

func main() {
	logLevel, _ := gocore.Config().Get("logLevel")
	logger := gocore.Log(progname, gocore.NewLogLevelFromString(logLevel))

	stats := gocore.Config().Stats()
	logger.Infof("STATS\n%s\nVERSION\n-------\n%s (%s)\n\n", stats, version, commit)

	go func() {
		profilerAddr, ok := gocore.Config().Get("blocktx_profilerAddr")
		if ok {
			logger.Infof("Starting profile on http://%s/debug/pprof", profilerAddr)
			logger.Fatalf("%v", http.ListenAndServe(profilerAddr, nil))
		}
	}()

	shutdown, err := cmd.StartBlockTx(logger)
	if err != nil {
		logger.Fatalf("Error starting blocktx: %v", err)
	}

	// setup signal catching
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	<-signalChan

	appCleanup(logger, shutdown)
	os.Exit(1)
}

func appCleanup(logger utils.Logger, shutdown func()) {
	logger.Infof("Shutting down...")
	shutdown()
}
