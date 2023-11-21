package cmd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/bitcoin-sv/arc/asynccaller"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	blockTxStore "github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/callbacker"
	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/metamorph/store/badger"
	"github.com/bitcoin-sv/arc/metamorph/store/dynamodb"
	"github.com/bitcoin-sv/arc/metamorph/store/sql"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/safemap"
	"github.com/ordishs/gocore"
	"github.com/spf13/viper"
)

const (
	DbModeBadger                   = "badger"
	DbModeDynamoDB                 = "dynamodb"
	dataRetentionPeriodDaysDefault = 14
)

func StartMetamorph(logger *slog.Logger) (func(), error) {
	folder := viper.GetString("dataFolder")
	if folder == "" {
		return nil, errors.New("dataFolder not found in config")
	}

	if err := os.MkdirAll(folder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data folder %s: %v", folder, err)
	}

	dbMode := viper.GetString("metamorph.db.mode")
	if dbMode == "" {
		return nil, errors.New("metamorph.db.mode not found in config")
	}

	s, err := NewStore(dbMode, folder)
	if err != nil {
		return nil, fmt.Errorf("failed to create metamorph store: %v", err)
	}

	blocktxAddress := viper.GetString("blocktx.dialAddr")
	if blocktxAddress == "" {
		return nil, errors.New("blocktx.dialAddr not found in config")
	}

	blockTxLogger, err := config.NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to create new logger: %v", err)
	}

	conn, err := blocktx.DialGRPC(blocktxAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	btx := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(conn), blocktx.WithLogger(blockTxLogger))

	metamorphGRPCListenAddress := viper.GetString("metamorph.listenAddr")
	if metamorphGRPCListenAddress == "" {
		return nil, errors.New("no metamorph.listenAddr setting found")
	}

	source := "-"
	if dbMode != DbModeDynamoDB {
		ip, port, err := net.SplitHostPort(metamorphGRPCListenAddress)
		if err != nil {
			return nil, fmt.Errorf("cannot parse ip address: %v", err)
		}

		if ip != "" {
			source = metamorphGRPCListenAddress
		} else {
			hint := viper.GetString("ipAddressHint")
			ips, err := utils.GetIPAddressesWithHint(hint)
			if err != nil {
				return nil, fmt.Errorf("cannot get local ip address")
			}

			if len(ips) != 1 {
				return nil, fmt.Errorf("cannot determine local ip address [%v]", ips)
			}

			source = fmt.Sprintf("%s:%s", ips[0], port)
		}

		return nil, fmt.Errorf("instance will register transactions with location %q", source)
	}

	pm, statusMessageCh, err := initPeerManager(logger, s)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize peer manager: %v", err)
	}

	callbackerAddress := viper.GetString("callbacker.dialAddr")
	if callbackerAddress == "" {
		return nil, errors.New("no callbacker.dialAddr setting found")
	}
	cb := callbacker.NewClient(callbackerAddress)

	callbackRegisterPath, err := filepath.Abs(path.Join(folder, "callback-register"))
	if err != nil {
		return nil, fmt.Errorf("could not get absolute path: %v", err)
	}

	// create an async caller to callbacker
	var metamorphLogger = gocore.Log("mtm", gocore.NewLogLevelFromString("INFO"))
	var cbAsyncCaller *asynccaller.AsyncCaller[callbacker_api.Callback]
	cbAsyncCaller, err = asynccaller.New[callbacker_api.Callback](
		metamorphLogger,
		callbackRegisterPath,
		10*time.Second,
		metamorph.NewRegisterCallbackClient(cb),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating async caller: %v", err)
	}

	mapExpiryStr := viper.GetString("metamorph.processorCacheExpiryTime")
	mapExpiry, err := time.ParseDuration(mapExpiryStr)
	if err != nil {
		return nil, fmt.Errorf("invalid metamorph.processorCacheExpiryTime: %s", mapExpiryStr)
	}

	processorLogger, err := config.NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %v", err)
	}

	metamorphProcessor, err := metamorph.NewProcessor(
		s,
		pm,
		cbAsyncCaller.GetChannel(),
		btx,
		metamorph.WithCacheExpiryTime(mapExpiry),
		metamorph.WithProcessorLogger(processorLogger),
		metamorph.WithLogFilePath(viper.GetString("metamorph.log.file")),
		metamorph.WithDataRetentionPeriod(time.Duration(viper.GetInt("metamorph.dataRetentionPeriodDays"))*24*time.Hour),
	)

	http.HandleFunc("/pstats", metamorphProcessor.HandleStats)

	go func() {
		for message := range statusMessageCh {
			_, err = metamorphProcessor.SendStatusForTransaction(message.Hash, message.Status, message.Peer, message.Err)
			if err != nil {
				logger.Error("Could not send status for transaction", slog.String("hash", message.Hash.String()), slog.String("err", err.Error()))
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
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				logger.Error("Could not get block processed status", slog.String("hash", hash.String()), slog.String("err", err.Error()))
				continue
			}

			// check whether we have already processed this block
			if processedAt == nil || processedAt.IsZero() {
				// process the block
				processBlock(logger, btx, metamorphProcessor, s, &blocktx_api.BlockAndSource{
					Hash:   block.Hash,
					Source: source,
				})
			}

			// check whether we have already processed the previous block
			blockHash, _ := chainhash.NewHash(block.PreviousHash)

			processedAt, err = s.GetBlockProcessed(context.Background(), blockHash)
			if err != nil {
				logger.Error("Could not get previous block processed status", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
				continue
			}
			if processedAt == nil || processedAt.IsZero() {
				// get the full previous block from block tx
				var previousBlock *blocktx_api.Block
				pHash, err := chainhash.NewHash(block.PreviousHash)
				if err != nil {
					logger.Error("Could not parse previous block hash", slog.String("hash", string(block.PreviousHash)), slog.String("err", err.Error()))
					continue
				}

				previousBlock, err = btx.GetBlock(context.Background(), pHash)
				if err != nil {
					if !errors.Is(err, blockTxStore.ErrBlockNotFound) {
						logger.Error("Could not get previous block from blocktx", slog.String("hash", pHash.String()), slog.String("err", err.Error()))
					}
					continue
				}

				// send the previous block to the process channel
				if previousBlock != nil {
					go func() {
						blockChan <- previousBlock
					}()
				}
			}
		}
	}()

	btx.Start(blockChan)
	opts := []metamorph.ServerOption{
		metamorph.WithLogger(logger),
	}

	if viper.GetBool("metamorph.checkUtxos") {
		peerRpcPassword := viper.GetString("peerRpc.password")
		if peerRpcPassword == "" {
			return nil, errors.New("setting peerRpc.password not found")
		}

		peerRpcUser := viper.GetString("peerRpc.user")
		if peerRpcUser == "" {
			return nil, errors.New("setting peerRpc.user not found")
		}

		peerRpcHost := viper.GetString("peerRpc.host")
		if peerRpcHost == "" {
			return nil, errors.New("setting peerRpc.host not found")
		}

		peerRpcPort := viper.GetInt("peerRpc.port")
		if peerRpcPort == 0 {
			return nil, errors.New("setting peerRpc.port not found")
		}

		rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpcUser, peerRpcPassword, peerRpcHost, peerRpcPort))
		if err != nil {
			return nil, fmt.Errorf("failed to parse rpc URL: %v", err)
		}

		node, err := bitcoin.NewFromURL(rpcURL, false)
		if err != nil {
			return nil, err
		}

		opts = append(opts, metamorph.WithForceCheckUtxos(node))
	}

	serv := metamorph.NewServer(s, metamorphProcessor, btx, source, opts...)

	go func() {
		grpcMessageSize := viper.GetInt("grpcMessageSize")
		if grpcMessageSize == 0 {
			logger.Error("grpcMessageSize must be set")
			return
		}
		if err = serv.StartGRPCServer(metamorphGRPCListenAddress, grpcMessageSize); err != nil {
			logger.Error("GRPCServer failed", slog.String("err", err.Error()))
		}
	}()

	peerSettings, err := blocktx.GetPeerSettings()
	if err != nil {
		return nil, fmt.Errorf("error getting peer settings: %v", err)
	}

	zmqCollector := safemap.New[string, *metamorph.ZMQStats]()

	for i, peerSetting := range peerSettings {
		zmqURL, err := peerSetting.GetZMQUrl()
		if err != nil {
			logger.Warn("failed to get zmq URL for peer", slog.Int("peer", i))
			continue
		}

		z := metamorph.NewZMQ(zmqURL, statusMessageCh)
		zmqCollector.Set(zmqURL.Host, z.Stats)
		port, err := strconv.Atoi(z.URL.Port())
		if err != nil {
			z.Logger.Fatalf("Could not parse port from metamorph_zmqAddress: %v", err)
		}

		z.Logger.Infof("Listening to ZMQ on %s:%d", z.URL.Hostname(), port)

		go z.Start(bitcoin.NewZMQ(z.URL.Hostname(), port, z.Logger))
	}

	// pass all the started peers to the collector
	_ = metamorph.NewZMQCollector(zmqCollector)

	return func() {
		logger.Info("Shutting down metamorph store")
		metamorphProcessor.Shutdown()
		err = s.Close(context.Background())
		if err != nil {
			logger.Error("Could not close store", slog.String("err", err.Error()))
		}
	}, nil
}

func NewStore(dbMode string, folder string) (s store.MetamorphStore, err error) {
	if dbMode == sql.DbModePostgres || dbMode == sql.DbModeSQLite || dbMode == sql.DbModeSQLiteM {
		s, err = sql.New(dbMode)
	} else {
		switch dbMode {
		case DbModeBadger:
			s, err = badger.New(path.Join(folder, "metamorph"))
			if err != nil {
				return nil, err
			}
		case DbModeDynamoDB:
			hostname, err := os.Hostname()
			if err != nil {
				return nil, err
			}

			ctx := context.Background()
			cfg, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithEC2IMDSRegion())
			if err != nil {
				return nil, err
			}

			ttlDays := viper.GetInt("metamorph.dataRetentionPeriodDays")
			if ttlDays == 0 {
				ttlDays = dataRetentionPeriodDaysDefault
			}

			s, err = dynamodb.New(
				awsdynamodb.NewFromConfig(cfg),
				hostname,
				time.Duration(ttlDays)*24*time.Hour,
			)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("db mode %s is invalid", dbMode)
		}
	}

	return s, err
}

func initPeerManager(logger *slog.Logger, s store.MetamorphStore) (p2p.PeerManagerI, chan *metamorph.PeerTxMessage, error) {
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
		return nil, nil, fmt.Errorf("unknown bitcoin_network: %s", networkStr)
	}

	messageCh := make(chan *metamorph.PeerTxMessage)
	var metamorphLogger = gocore.Log("mtm", gocore.NewLogLevelFromString("INFO"))
	pm := p2p.NewPeerManager(metamorphLogger, network)

	peerHandler := metamorph.NewPeerHandler(s, messageCh)

	peerSettings, err := blocktx.GetPeerSettings()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting peer settings: %v", err)
	}

	for _, peerSetting := range peerSettings {
		peerUrl, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, nil, fmt.Errorf("error getting peer url: %v", err)
		}

		var peer *p2p.Peer
		peer, err = p2p.NewPeer(metamorphLogger, peerUrl, peerHandler, network)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating peer %s: %v", peerUrl, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			return nil, nil, fmt.Errorf("error adding peer %s: %v", peerUrl, err)
		}
	}

	return pm, messageCh, nil
}

func processBlock(logger *slog.Logger, btc blocktx.ClientI, p metamorph.ProcessorI, s store.MetamorphStore, blockAndSource *blocktx_api.BlockAndSource) {
	mt, err := btc.GetMinedTransactionsForBlock(context.Background(), blockAndSource)
	if err != nil {
		logger.Error("Could not get mined transactions for block", slog.String("hash", hex.EncodeToString(bt.ReverseBytes(blockAndSource.Hash))), slog.String("err", err.Error()))
		return
	}

	logger.Info("Incoming BLOCK", slog.String("hash", hex.EncodeToString(bt.ReverseBytes(mt.Block.Hash))))

	for _, tx := range mt.Transactions {
		logger.Debug("Received MINED message from blocktx for transaction", slog.String("hash", hex.EncodeToString(bt.ReverseBytes(tx.Hash))))

		hash, _ := chainhash.NewHash(tx.Hash)
		blockHash, _ := chainhash.NewHash(mt.Block.Hash)

		_, err = p.SendStatusMinedForTransaction(hash, blockHash, mt.Block.Height)
		if err != nil {
			logger.Error("Could not send mined status for transaction", slog.String("hash", hex.EncodeToString(bt.ReverseBytes(tx.Hash))), slog.String("err", err.Error()))
			return
		}
	}

	logger.Info("Marked transactions as MINED", slog.Int("number", len(mt.Transactions)))

	hash, _ := chainhash.NewHash(blockAndSource.Hash)

	err = s.SetBlockProcessed(context.Background(), hash)
	if err != nil {
		logger.Error("Failed to set block processed status", slog.String("err", err.Error()))
	}
}
