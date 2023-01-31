package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/TAAL-GmbH/arc/asynccaller"
	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	blockTxStore "github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/callbacker"
	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/metamorph"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"github.com/pkg/errors"
)

func StartMetamorph(logger utils.Logger) (func(), error) {
	folder, _ := gocore.Config().Get("dataFolder", "data")
	if err := os.MkdirAll(folder, 0755); err != nil {
		logger.Fatalf("failed to create data folder %s: %+v", folder, err)
	}

	dbMode, _ := gocore.Config().Get("metamorph_dbMode", "sqlite")

	s, err := metamorph.NewStore(dbMode, folder)
	if err != nil {
		logger.Fatalf("Error creating metamorph store: %v", err)
	}

	address, _ := gocore.Config().Get("blocktxAddress") //, "localhost:8001")
	btc := blocktx.NewClient(logger, address)

	workerCount, _ := gocore.Config().GetInt("processorWorkerCount", 16)

	metamorphGRPCListenAddress, ok := gocore.Config().Get("metamorph_grpcAddress") //, "localhost:8000")
	if !ok {
		logger.Fatalf("no metamorph_grpcAddress setting found")
	}

	ip, port, err := net.SplitHostPort(metamorphGRPCListenAddress)
	if err != nil {
		logger.Fatalf("cannot parse ip address: %v", err)
	}

	var metamorphAddress string

	if ip != "" {
		metamorphAddress = metamorphGRPCListenAddress
	} else {
		hint, _ := gocore.Config().Get("ip_address_hint", "")
		ips, err := utils.GetIPAddressesWithHint(hint)
		if err != nil {
			logger.Fatalf("cannot get local ip address")
		}

		if len(ips) != 1 {
			logger.Fatalf("cannot determine local ip address [%v]", ips)
		}

		metamorphAddress = fmt.Sprintf("%s:%s", ips[0], port)
	}

	logger.Infof("Instance will register transactions with location %q", metamorphAddress)

	pm, peerMessageCh := initPeerManager(logger, s)

	txRegisterPath, err := filepath.Abs(path.Join(folder, "tx-register"))
	if err != nil {
		logger.Fatalf("Could not get absolute path: %v", err)
	}

	// create an async caller to store all the transaction registrations that cannot be sent to blocktx right away
	var asyncCaller *asynccaller.AsyncCaller[blocktx_api.TransactionAndSource]
	asyncCaller, err = asynccaller.New[blocktx_api.TransactionAndSource](
		logger,
		txRegisterPath,
		10*time.Second,
		metamorph.NewRegisterTransactionCallerClient(btc),
	)
	if err != nil {
		logger.Fatalf("error creating async caller: %v", err)
	}

	callbackerAddress, ok := gocore.Config().Get("callbacker_grpcAddress", "localhost:8002")
	if !ok {
		logger.Fatalf("no callbacker_grpcAddress setting found")
	}
	cb := callbacker.NewClient(logger, callbackerAddress)

	callbackRegisterPath, err := filepath.Abs(path.Join(folder, "callback-register"))
	if err != nil {
		logger.Fatalf("Could not get absolute path: %v", err)
	}

	// create an async caller to callbacker
	var cbAsyncCaller *asynccaller.AsyncCaller[callbacker_api.Callback]
	cbAsyncCaller, err = asynccaller.New[callbacker_api.Callback](
		logger,
		callbackRegisterPath,
		10*time.Second,
		metamorph.NewRegisterCallbackClient(cb),
	)
	if err != nil {
		logger.Fatalf("error creating async caller: %v", err)
	}

	metamorphProcessor := metamorph.NewProcessor(
		workerCount,
		s,
		pm,
		metamorphAddress,
		asyncCaller.GetChannel(),
		cbAsyncCaller.GetChannel(),
	)

	go func() {
		for message := range peerMessageCh {
			processed, err := metamorphProcessor.SendStatusForTransaction(message.Txid, metamorph_api.Status(message.Status), message.Err)
			if processed {
				logger.Debugf("Status change reported: %s: %s", message.Txid, metamorph_api.Status(message.Status))
			}
			if err != nil {
				logger.Errorf("Could not send status for transaction %s: %v", message.Txid, err)
			}

		}
	}()

	if gocore.Config().GetBool("stats_keypress", false) {
		// The double invocation is the get PrintStatsOnKeypress to start and return a function
		// that can be deferred to reset the TTY when the program exits.
		defer metamorphProcessor.PrintStatsOnKeypress()()
	}

	go func() {
		// load all transactions into memory from disk that have not been seen on the network
		// this will make sure they are re-broadcast until a response is received
		metamorphProcessor.LoadUnseen()
	}()

	// create a channel to receive mined block messages from the block tx service
	var blockChan = make(chan *blocktx_api.Block)
	go func() {
		var processedAt *time.Time
		for block := range blockChan {
			processedAt, err = s.GetBlockProcessed(context.Background(), block.Hash)
			if err != nil {
				logger.Errorf("Could not get block processed status: %v", err)
				continue
			}

			// check whether we have already processed this block
			if processedAt == nil || processedAt.IsZero() {
				// process the block
				processBlock(logger, btc, metamorphProcessor, s, &blocktx_api.BlockAndSource{
					Hash:   block.Hash,
					Source: metamorphAddress,
				})
			}

			// check whether we have already processed the previous block
			processedAt, err = s.GetBlockProcessed(context.Background(), block.PreviousHash)
			if err != nil {
				logger.Errorf("Could not get previous block processed status: %v", err)
				continue
			}
			if processedAt == nil || processedAt.IsZero() {
				// get the full previous block from block tx
				var previousBlock *blocktx_api.Block
				previousBlock, err = btc.GetBlock(context.Background(), block.PreviousHash)
				if err != nil {
					if !errors.Is(err, blockTxStore.ErrBlockNotFound) {
						logger.Errorf("Could not get previous block from block tx: %v", err)
					}
					continue
				}

				// send the previous block to the process channel
				if previousBlock == nil {
					go func() {
						blockChan <- previousBlock
					}()
				}
			}
		}
	}()

	go btc.Start(blockChan)

	serv := metamorph.NewServer(logger, s, metamorphProcessor)

	go func() {
		if err = serv.StartGRPCServer(metamorphGRPCListenAddress); err != nil {
			logger.Errorf("GRPCServer failed: %v", err)
		}
	}()

	return func() {
		logger.Infof("Shutting down metamorph store")
		err = s.Close(context.Background())
		if err != nil {
			logger.Errorf("Could not close store: %v", err)
		}
	}, nil
}

func initPeerManager(logger utils.Logger, s store.MetamorphStore) (p2p.PeerManagerI, chan *metamorph.PeerTxMessage) {
	network := wire.TestNet
	if gocore.Config().GetBool("mainnet", false) {
		network = wire.MainNet
	}

	messageCh := make(chan *metamorph.PeerTxMessage)
	pm := p2p.NewPeerManager(logger, network)

	peerHandler := metamorph.NewPeerHandler(s, messageCh)

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

		if err = pm.AddPeer(p2pURL.Host, peerHandler); err != nil {
			logger.Fatalf("error adding peer %s: %v", p2pURL.Host, err)
		}
	}

	return pm, messageCh
}

func processBlock(logger utils.Logger, btc blocktx.ClientI, p metamorph.ProcessorI, s store.MetamorphStore, blockAndSource *blocktx_api.BlockAndSource) {
	mt, err := btc.GetMinedTransactionsForBlock(context.Background(), blockAndSource)
	if err != nil {
		logger.Errorf("Could not get mined transactions for block %x: %v", bt.ReverseBytes(blockAndSource.Hash), err)
		return
	}

	logger.Infof("Incoming BLOCK %x", bt.ReverseBytes(mt.Block.Hash))

	for _, tx := range mt.Transactions {
		logger.Debugf("Received MINED message from BlockTX for transaction %x", bt.ReverseBytes(tx.Hash))

		_, err = p.SendStatusMinedForTransaction(tx.Hash, mt.Block.Hash, int32(mt.Block.Height))
		if err != nil {
			logger.Errorf("Could not send mined status for transaction %x: %v", bt.ReverseBytes(tx.Hash), err)
			return
		}
	}

	err = s.SetBlockProcessed(context.Background(), blockAndSource.Hash)
	if err != nil {
		logger.Errorf("Could not set block processed status: %v", err)
	}
}
