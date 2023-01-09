package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store/sql"
	"github.com/TAAL-GmbH/arc/callbacker"
	callbackerBadgerhold "github.com/TAAL-GmbH/arc/callbacker/store/badgerhold"
	"github.com/TAAL-GmbH/arc/metamorph"
	"github.com/TAAL-GmbH/arc/metamorph/store/badgerhold"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/libsv/go-bt/v2"
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
	//////
	// BlockTX
	//////
	blockStore, err := sql.NewSQLStore("sqlite")
	if err != nil {
		panic("Could not connect to fn: " + err.Error())
	}

	blockNotifier := blocktx.NewBlockNotifier(blockStore, logger)

	blockTxServer := blocktx.NewServer(blockStore, blockNotifier, logger)

	go func() {
		if err := blockTxServer.StartGRPCServer(); err != nil {
			logger.Fatal(err)
		}
	}()

	//////
	// Callbacker
	//////
	callbackStore, err := callbackerBadgerhold.New("data_callbacker", 2*time.Minute)
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

	go func() {
		err = srv.StartGRPCServer()
		if err != nil {
			logger.Fatal(err)
		}
	}()

	//////
	// Metamorph
	//////
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

	address, _ := gocore.Config().Get("blocktxAddress") //, "localhost:8001")
	btc := blocktx.NewClient(logger, address)

	workerCount, _ := gocore.Config().GetInt("processorWorkerCount", 10)

	metamorphAddress, ok := gocore.Config().Get("metamorph_grpcAddress", "localhost:8000")
	if !ok {
		logger.Fatalf("no metamorph_grpcAddress setting found")
	}

	metamorphProcessor := metamorph.NewProcessor(workerCount, s, pm, metamorphAddress, btc)

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

	// create a channel to receive mined block messages from the block tx service
	var blockChan = make(chan *blocktx_api.Block)
	go func() {
		for block := range blockChan {
			processBlock(btc, metamorphProcessor, &blocktx_api.BlockAndSource{
				Hash:   block.Hash,
				Source: metamorphAddress,
			})

			// For catchup, process the previous block hash too...
			// TODO - don't do this is we have already seen the previous block
			processBlock(btc, metamorphProcessor, &blocktx_api.BlockAndSource{
				Hash:   block.PreviousHash,
				Source: metamorphAddress,
			})
		}
	}()

	go btc.Start(blockChan)

	serv := metamorph.NewServer(logger, s, metamorphProcessor)
	if err = serv.StartGRPCServer(metamorphAddress); err != nil {
		logger.Errorf("GRPCServer failed: %v", err)
	}
}

func processBlock(btc blocktx.ClientI, p metamorph.ProcessorI, blockAndSource *blocktx_api.BlockAndSource) {
	mt, err := btc.GetMinedTransactionsForBlock(context.Background(), blockAndSource)
	if err != nil {
		logger.Errorf("Could not get mined transactions for block %x: %v", bt.ReverseBytes(blockAndSource.Hash), err)
		return
	}

	for _, tx := range mt.Transactions {
		logger.Infof("Received MINED message from BlockTX for transaction %x", bt.ReverseBytes(tx.Hash))

		_, err := p.SendStatusMinedForTransaction(tx.Hash, mt.Block.Hash, int32(mt.Block.Height))
		if err != nil {
			logger.Errorf("Could not send mined status for transaction %x: %v", bt.ReverseBytes(tx.Hash), err)
		}
	}
}
