package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/bitcoin-sv/arc/asynccaller"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	blockTxStore "github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/callbacker"
	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/safemap"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func StartMetamorph(logger utils.Logger) (func(), error) {
	folder := viper.GetString("dataFolder")
	if folder == "" {
		return nil, errors.New("dataFolder not found in config")
	}

	if err := os.MkdirAll(folder, 0755); err != nil {
		logger.Fatalf("failed to create data folder %s: %+v", folder, err)
	}

	dbMode := viper.GetString("metamorph.db.mode")
	if dbMode == "" {
		return nil, errors.New("metamorph.db.mode not found in config")
	}

	s, err := metamorph.NewStore(dbMode, folder)
	if err != nil {
		logger.Fatalf("Error creating metamorph store: %v", err)
	}

	address := viper.GetString("blocktx.dialAddr")
	if address == "" {
		return nil, errors.New("blocktx.dialAddr not found in config")
	}

	btc := blocktx.NewClient(logger, address)

	metamorphGRPCListenAddress := viper.GetString("metamorph.listenAddr")
	if metamorphGRPCListenAddress == "" {
		logger.Fatalf("no metamorph.listenAddr setting found")
	}

	ip, port, err := net.SplitHostPort(metamorphGRPCListenAddress)
	if err != nil {
		logger.Fatalf("cannot parse ip address: %v", err)
	}

	var source string

	if ip != "" {
		source = metamorphGRPCListenAddress
	} else {
		hint := viper.GetString("ipAddressHint")
		ips, err := utils.GetIPAddressesWithHint(hint)
		if err != nil {
			logger.Fatalf("cannot get local ip address")
		}

		if len(ips) != 1 {
			logger.Fatalf("cannot determine local ip address [%v]", ips)
		}

		source = fmt.Sprintf("%s:%s", ips[0], port)
	}

	logger.Infof("Instance will register transactions with location %q", source)

	pm, statusMessageCh := initPeerManager(logger, s)

	callbackerAddress := viper.GetString("callbacker.dialAddr")
	if callbackerAddress == "" {
		logger.Fatalf("no callbacker.dialAddr setting found")
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
		s,
		pm,
		source,
		cbAsyncCaller.GetChannel(),
		btc,
	)

	http.HandleFunc("/pstats", metamorphProcessor.HandleStats)

	go func() {
		for message := range statusMessageCh {
			_, err = metamorphProcessor.SendStatusForTransaction(message.Hash, message.Status, message.Peer, message.Err)
			if err != nil {
				logger.Errorf("Could not send status for transaction %v: %v", message.Hash, err)
			}
		}
	}()

	if viper.GetBool("metamorph.statsKeypress") {
		// The double invocation is the get PrintStatsOnKeypress to start and return a function
		// that can be deferred to reset the TTY when the program exits.
		defer metamorphProcessor.PrintStatsOnKeypress()()
	}

	go func() {
		// load all transactions into memory from disk that have not been seen on the network
		// this will make sure they are re-broadcast until a response is received
		metamorphProcessor.LoadUnmined()
	}()

	// create a channel to receive mined block messages from the block tx service
	var blockChan = make(chan *blocktx_api.Block)
	go func() {
		var processedAt *time.Time
		for block := range blockChan {
			hash, _ := chainhash.NewHash(block.Hash[:])
			processedAt, err = s.GetBlockProcessed(context.Background(), hash)
			if err != nil {
				logger.Errorf("Could not get block processed status: %v", err)
				continue
			}

			// check whether we have already processed this block
			if processedAt == nil || processedAt.IsZero() {
				// process the block
				processBlock(logger, btc, metamorphProcessor, s, &blocktx_api.BlockAndSource{
					Hash:   block.Hash,
					Source: source,
				})
			}

			// check whether we have already processed the previous block
			blockHash, _ := chainhash.NewHash(block.PreviousHash)

			processedAt, err = s.GetBlockProcessed(context.Background(), blockHash)
			if err != nil {
				logger.Errorf("Could not get previous block processed status: %v", err)
				continue
			}
			if processedAt == nil || processedAt.IsZero() {
				// get the full previous block from block tx
				var previousBlock *blocktx_api.Block
				pHash, err := chainhash.NewHash(block.PreviousHash)
				if err != nil {
					logger.Errorf("Could not get previous block hash: %v", err)
					continue
				}

				previousBlock, err = btc.GetBlock(context.Background(), pHash)
				if err != nil {
					if !errors.Is(err, blockTxStore.ErrBlockNotFound) {
						logger.Errorf("Could not get previous block with hash %s from block tx: %v", pHash.String(), err)
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

	serv := metamorph.NewServer(logger, s, metamorphProcessor, btc, source)

	go func() {
		grpcMessageSize := viper.GetInt("grpcMessageSize")
		if grpcMessageSize == 0 {
			logger.Errorf("grpcMessageSize must be set")
			return
		}
		if err = serv.StartGRPCServer(metamorphGRPCListenAddress, grpcMessageSize); err != nil {
			logger.Errorf("GRPCServer failed: %v", err)
		}
	}()

	peerSettings, err := blocktx.GetPeerSettings()
	if err != nil {
		logger.Fatalf("error getting peer settings: %v", err)
	}

	zmqCollector := safemap.New[string, *metamorph.ZMQStats]()

	for i, peerSetting := range peerSettings {
		zmqURL, err := peerSetting.GetZMQUrl()
		if err != nil {
			logger.Warnf("failed to get zmq URL for peer %d", i)
			continue
		}

		z := metamorph.NewZMQ(zmqURL, statusMessageCh)
		zmqCollector.Set(zmqURL.Host, z.Stats)
		go z.Start()
	}

	// pass all the started peers to the collector
	_ = metamorph.NewZMQCollector(zmqCollector)

	return func() {
		logger.Infof("Shutting down metamorph store")
		err = s.Close(context.Background())
		if err != nil {
			logger.Errorf("Could not close store: %v", err)
		}
	}, nil
}

func initPeerManager(logger utils.Logger, s store.MetamorphStore) (p2p.PeerManagerI, chan *metamorph.PeerTxMessage) {
	networkStr := viper.GetString("network")

	var network wire.BitcoinNet

	switch networkStr {
	case "mainnet":
		network = wire.MainNet
	case "testnet":
		network = wire.TestNet3
	case "regtest":
		network = wire.TestNet
	default:
		logger.Fatalf("unknown bitcoin_network: %s", networkStr)
	}

	logger.Infof("Assuming bitcoin network is %s", network)

	messageCh := make(chan *metamorph.PeerTxMessage)
	pm := p2p.NewPeerManager(logger, network)

	peerHandler := metamorph.NewPeerHandler(s, messageCh)

	peerSettings, err := blocktx.GetPeerSettings()
	if err != nil {
		logger.Fatalf("error getting peer settings: %v", err)
	}

	for _, peerSetting := range peerSettings {
		peerUrl, err := peerSetting.GetP2PUrl()
		if err != nil {
			logger.Fatalf("error getting peer url: %v", err)
		}

		var peer *p2p.Peer
		peer, err = p2p.NewPeer(logger, peerUrl, peerHandler, network)
		if err != nil {
			logger.Fatalf("error creating peer %s: %v", peerUrl, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			logger.Fatalf("error adding peer %s: %v", peerUrl, err)
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

		hash, _ := chainhash.NewHash(tx.Hash)
		blockHash, _ := chainhash.NewHash(mt.Block.Hash)

		_, err = p.SendStatusMinedForTransaction(hash, blockHash, mt.Block.Height)
		if err != nil {
			logger.Errorf("Could not send mined status for transaction %x: %v", bt.ReverseBytes(tx.Hash), err)
			return
		}
	}

	logger.Infof("Marked %d transactions as MINED", len(mt.Transactions))

	hash, _ := chainhash.NewHash(blockAndSource.Hash)

	err = s.SetBlockProcessed(context.Background(), hash)
	if err != nil {
		logger.Errorf("Could not set block processed status: %v", err)
	}
}
