package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store/sql"
	"github.com/TAAL-GmbH/arc/metamorph"
	"github.com/TAAL-GmbH/arc/metamorph/store/badgerhold"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/libsv/go-bt/v2"
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

	peer1Url, err, found := gocore.Config().GetURL("peer_1_rpc")
	if !found {
		logger.Fatal("No peer_1_rpc setting found.")
	}
	if err != nil {
		logger.Fatal(err)
	}

	b, err := bitcoin.NewFromURL(peer1Url, false)
	if err != nil {
		logger.Fatalf("Could not connect to bitcoin: %v", err)
	}

	blockTxProcessor, err := blocktx.NewBlockTxProcessor(blockStore, b)
	if err != nil {
		logger.Fatal(err)
	}

	blockTxProcessor.Start()

	blockTxServer := blocktx.NewServer(blockStore, blockTxProcessor, logger)

	go func() {
		if err := blockTxServer.StartGRPCServer(); err != nil {
			logger.Fatal(err)
		}
	}()

	s, bErr := badgerhold.New("")
	if bErr != nil {
		logger.Fatalf("Error creating metamorph store: %v", bErr)
	}

	messageCh := make(chan *p2p.PMMessage)

	pm := p2p.NewPeerManager(messageCh)

	peerStore := metamorph.NewMetamorphPeerStore(s)

	peerCount, _ := gocore.Config().GetInt("peerCount", 0)
	if peerCount == 0 {
		logger.Fatalf("peerCount must be set")
	}

	for i := 1; i <= peerCount; i++ {
		p2pURL, err, found := gocore.Config().GetURL(fmt.Sprintf("peer_%d_p2p", i))
		if !found {
			logger.Fatalf("peer_%d_p2p must be set", i)
		}
		if err != nil {
			logger.Fatalf("error reading peer_%d_p2p: %v", i, err)
		}

		if err := pm.AddPeer(p2pURL.Host, peerStore); err != nil {
			logger.Fatalf("error adding peer %s: %v", p2pURL.Host, err)
		}
	}

	workerCount, _ := gocore.Config().GetInt("processorWorkerCount", 10)

	metamorphProcessor := metamorph.NewProcessor(workerCount, s, pm)

	go func() {
		for message := range messageCh {
			logger.Infof("Status change reported: %s: %s", message.Txid, message.Status)
			_, err = metamorphProcessor.SendStatusForTransaction(message.Txid, message.Status, message.Err)
			if err != nil {
				logger.Errorf("Could not send status for transaction %s: %v", message.Txid, err)
			}
		}
	}()

	// The ZMQ listener is not needed any more, when connected twice to a node,
	// or to at least one node that is not getting transactions sent to it
	// z := metamorph.NewZMQ(metamorphProcessor)
	// go z.Start()

	// The double invocation is the get PrintStatsOnKeypress to start and return a function
	// that can be deferred to reset the TTY when the program exits.
	defer metamorphProcessor.PrintStatsOnKeypress()()

	go func() {
		// load all transactions into memory from disk that have not been seen on the network
		// this will make sure they are re-broadcast until a response is received
		// TODO should this be synchronous instead of in a goroutine?
		metamorphProcessor.LoadUnseen()
	}()

	// create a channel to receive mined tx messages from the block tx service
	var minedBlockChan = make(chan *blocktx_api.MinedTransaction)
	go func() {
		for mt := range minedBlockChan {
			for _, tx := range mt.Txs {
				logger.Infof("Received MINED message from P2P %x", bt.ReverseBytes(tx.Hash))
				_, err = metamorphProcessor.SendStatusMinedForTransaction(tx.Hash, mt.Block.Hash, int32(mt.Block.Height))
				if err != nil {
					logger.Errorf("Could not send mined status for transaction %x: %v", bt.ReverseBytes(tx.Hash), err)
				}
			}
		}
	}()

	address, _ := gocore.Config().Get("blocktxAddress") //, "localhost:8001")
	btc := blocktx.NewClient(logger, address)
	go btc.Start(minedBlockChan)

	serv := metamorph.NewServer(logger, s, metamorphProcessor)
	if err = serv.StartGRPCServer(); err != nil {
		logger.Errorf("GRPCServer failed: %v", err)
	}
}
