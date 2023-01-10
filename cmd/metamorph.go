package cmd

import (
	"context"
	"fmt"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/metamorph"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store/badgerhold"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/gocore"
)

const ISO8601 = "2006-01-02T15:04:05.999Z"

func StartMetamorph(logger *gocore.Logger) {
	s, err := badgerhold.New("")
	if err != nil {
		logger.Fatalf("Error creating metamorph store: %v", err)
	}

	address, _ := gocore.Config().Get("blocktxAddress") //, "localhost:8001")
	btc := blocktx.NewClient(logger, address)

	workerCount, _ := gocore.Config().GetInt("processorWorkerCount", 10)

	metamorphAddress, ok := gocore.Config().Get("metamorph_grpcAddress", "localhost:8000")
	if !ok {
		logger.Fatalf("no metamorph_grpcAddress setting found")
	}

	pm, peerMessageCh := initPeerManager(logger, s)

	// create a batcher to store all the transaction registrations that cannot be sent to blocktx right away
	registerCh := initRegisterTransactionChannel(logger, btc)

	metamorphProcessor := metamorph.NewProcessor(workerCount, s, pm, metamorphAddress, registerCh)

	go func() {
		for message := range peerMessageCh {
			logger.Infof("Status change reported: %s: %s", message.Txid, metamorph_api.Status(message.Status))
			_, err = metamorphProcessor.SendStatusForTransaction(message.Txid, metamorph_api.Status(message.Status), message.Err)
			if err != nil {
				logger.Errorf("Could not send status for transaction %s: %v", message.Txid, err)
			}
		}
	}()

	// The double invocation is the get PrintStatsOnKeypress to start and return a function
	// that can be deferred to reset the TTY when the program exits.
	defer metamorphProcessor.PrintStatsOnKeypress()()

	go func() {
		// load all transactions into memory from disk that have not been seen on the network
		// this will make sure they are re-broadcast until a response is received
		metamorphProcessor.LoadUnseen()
	}()

	// create a channel to receive mined block messages from the block tx service
	var blockChan = make(chan *blocktx_api.Block)
	go func() {
		for block := range blockChan {
			processBlock(logger, btc, metamorphProcessor, &blocktx_api.BlockAndSource{
				Hash:   block.Hash,
				Source: metamorphAddress,
			})

			// For catchup, process the previous block hash too...
			// TODO - don't do this is we have already seen the previous block
			processBlock(logger, btc, metamorphProcessor, &blocktx_api.BlockAndSource{
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

func initPeerManager(logger *gocore.Logger, s *badgerhold.BadgerHold) (p2p.PeerManagerI, chan *p2p.PMMessage) {
	network := wire.TestNet
	if gocore.Config().GetBool("mainnet", false) {
		network = wire.MainNet
	}

	messageCh := make(chan *p2p.PMMessage)
	pm := p2p.NewPeerManager(logger, messageCh, network)

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

		if err = pm.AddPeer(p2pURL.Host, peerStore); err != nil {
			logger.Fatalf("error adding peer %s: %v", p2pURL.Host, err)
		}
	}

	return pm, messageCh
}

func processBlock(logger *gocore.Logger, btc blocktx.ClientI, p metamorph.ProcessorI, blockAndSource *blocktx_api.BlockAndSource) {
	mt, err := btc.GetMinedTransactionsForBlock(context.Background(), blockAndSource)
	if err != nil {
		logger.Errorf("Could not get mined transactions for block %x: %v", bt.ReverseBytes(blockAndSource.Hash), err)
		return
	}

	logger.Infof("Incoming BLOCK %x", bt.ReverseBytes(mt.Block.Hash))

	for _, tx := range mt.Transactions {
		logger.Infof("Received MINED message from BlockTX for transaction %x", bt.ReverseBytes(tx.Hash))

		_, err = p.SendStatusMinedForTransaction(tx.Hash, mt.Block.Hash, int32(mt.Block.Height))
		if err != nil {
			logger.Errorf("Could not send mined status for transaction %x: %v", bt.ReverseBytes(tx.Hash), err)
		}
	}
}
