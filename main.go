package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/store/sql"
	"github.com/ordishs/go-bitcoin"
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

	// setup signal catching
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, os.Interrupt)

	go func() {
		<-signalChan

		appCleanup()
		os.Exit(1)
	}()

	start()
}

func appCleanup() {
	logger.Infof("Shutting down...")
}

func start() {
	blockStore, err := sql.NewSQLStore("sqlite")
	if err != nil {
		panic("Could not connect to fn: " + err.Error())
	}

	// dbConn, err := memory.NewClient()
	// if err != nil {
	// 	logger.Fatal(err)
	// }

	host, _ := gocore.Config().Get("bitcoinHost", "localhost")
	port, _ := gocore.Config().GetInt("rpcPort", 8332)
	username, _ := gocore.Config().Get("rpcUsername", "bitcoin")
	password, _ := gocore.Config().Get("rpcPassword", "bitcoin")

	b, err := bitcoin.New(host, port, username, password, false)
	if err != nil {
		logger.Fatalf("Could not connect to bitcoin: %v", err)
	}

	var p *blocktx.Processor
	p, err = blocktx.NewBlockTxProcessor(blockStore, b)
	if err != nil {
		logger.Fatal(err)
	}

	p.Start()

	srv := blocktx.NewServer(blockStore, p, logger)

	err = srv.StartGRPCServer()
	if err != nil {
		logger.Fatal(err)
	}
}
