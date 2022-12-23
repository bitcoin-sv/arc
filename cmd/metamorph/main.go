package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/metamorph"
	"github.com/TAAL-GmbH/arc/metamorph/store/badgerhold"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/gocore"
)

// Name used by build script for the binaries. (Please keep on single line)
const progname = "github.com/TAAL-GmbH/arc"

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
		profilerAddr, ok := gocore.Config().Get("metamorph_profilerAddr")
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
	messageCh := make(chan *p2p.PMMessage)

	s := badgerhold.New()

	pm := p2p.NewPeerManager(s, messageCh)

	workerCount, _ := gocore.Config().GetInt("processorWorkerCount", 10)

	p := metamorph.NewProcessor(workerCount, s, pm)

	go func() {
		for message := range messageCh {
			_, err := p.SendStatusForTransaction(message.Txid, message.Status, message.Err)
			if err != nil {
				logger.Errorf("Error sending status for transaction %s: %v", message.Txid, err)
			}
		}
	}()

	z := metamorph.NewZMQ(p)
	go z.Start()

	// The double invocation is the get PrintStatsOnKeypress to start and return a function
	// that can be deferred to reset the TTY when the program exits.
	defer p.PrintStatsOnKeypress()()

	go func() {
		// load all transactions into memory from disk that have not been seen on the network
		// this will make sure they are re-broadcast until a response is received
		// TODO should this be synchronous instead of in a goroutine?
		p.LoadUnseen()
	}()

	// create a channel to receive mined tx messages from the block tx service
	var minedBlockChan = make(chan *blocktx_api.MinedTransaction)
	go func() {
		for mt := range minedBlockChan {
			for _, tx := range mt.Txs {
				_, err := p.SendStatusMinedForTransaction(tx.Hash, mt.Block.Hash, int32(mt.Block.Height))
				if err != nil {
					logger.Errorf("Could not send mined status for transaction %x: %v", bt.ReverseBytes(tx.Hash), err)
				}
			}
		}
	}()

	address, _ := gocore.Config().Get("blocktxAddress") //, "localhost:8001")
	btc := blocktx.NewClient(logger, address)
	go btc.Start(minedBlockChan)

	serv := metamorph.NewServer(logger, s, p)
	if err := serv.StartGRPCServer(); err != nil {
		logger.Errorf("GRPCServer failed: %v", err)
	}
}
