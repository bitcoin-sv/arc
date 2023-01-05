package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/TAAL-GmbH/arc/callbacker"
	"github.com/TAAL-GmbH/arc/callbacker/store/badgerhold"
	"github.com/ordishs/gocore"

	_ "github.com/lib/pq"
)

// Name used by build script for the binaries. (Please keep on single line)
const progname = "callbacker"

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
		profilerAddr, ok := gocore.Config().Get("callbacker_profilerAddr")
		if ok {
			logger.Infof("Starting profile on http://%s/debug/pprof", profilerAddr)
			logger.Fatalf("%v", http.ListenAndServe(profilerAddr, nil))
		}
	}()

	// setup signal catching
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, os.Interrupt)

	shutdown := start()

	go func() {
		<-signalChan

		appCleanup(shutdown)
		os.Exit(1)
	}()
}

func appCleanup(shutdown func()) {
	logger.Infof("Shutting down...")
	shutdown()
}

func start() func() {
	callbackStore, err := badgerhold.New("data_callbacker", 2*time.Minute)
	if err != nil {
		logger.Fatal(err)
	}

	var callbackWorker *callbacker.Callbacker
	callbackWorker, err = callbacker.NewCallbacker(callbackStore)
	if err != nil {
		logger.Fatal(err)
	}
	callbackWorker.Start()

	srv := callbacker.NewServer(logger, callbackWorker)
	err = srv.StartGRPCServer()
	if err != nil {
		logger.Fatal(err)
	}

	return func() {
		callbackWorker.Stop()
	}
}
