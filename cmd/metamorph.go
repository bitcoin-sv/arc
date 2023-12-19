package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	blockTxStore "github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/metamorph/store/badger"
	"github.com/bitcoin-sv/arc/metamorph/store/dynamodb"
	"github.com/bitcoin-sv/arc/metamorph/store/postgresql"
	"github.com/bitcoin-sv/arc/metamorph/store/sqlite"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/safemap"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	DbModeBadger     = "badger"
	DbModeDynamoDB   = "dynamodb"
	DbModePostgres   = "postgres"
	DbModeSQLiteM    = "sqlite_memory"
	DbModeSQLite     = "sqlite"
	unminedTxsPeriod = 2 * time.Minute
)

func StartMetamorph(logger utils.Logger) (func(), error) {
	dbMode := viper.GetString("metamorph.db.mode")
	if dbMode == "" {
		return nil, errors.New("metamorph.db.mode not found in config")
	}
	folder := viper.GetString("dataFolder")
	if folder == "" {
		return nil, errors.New("dataFolder not found in config")
	}

	if err := os.MkdirAll(folder, 0750); err != nil {
		return nil, fmt.Errorf("failed to create data folder %s: %+v", folder, err)
	}

	s, err := NewStore(dbMode, folder)
	if err != nil {
		logger.Fatalf("Error creating metamorph store: %v", err)
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
		logger.Fatalf("no metamorph.listenAddr setting found")
	}

	source := metamorphGRPCListenAddress
	if viper.GetBool("metamorph.network.fixedIp") {
		ip, port, err := net.SplitHostPort(metamorphGRPCListenAddress)
		if err != nil {
			logger.Fatalf("cannot parse ip address: %v", err)
		}

		if ip != "" {
			source = metamorphGRPCListenAddress
		} else {
			hint := viper.GetString("metamorph.network.ipAddressHint")
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
	}

	pm, statusMessageCh := initPeerManager(logger, s)

	mapExpiryStr := viper.GetString("metamorph.processorCacheExpiryTime")
	mapExpiry, err := time.ParseDuration(mapExpiryStr)
	if err != nil {
		logger.Errorf("Invalid metamorph.processorCacheExpiryTime: %s", mapExpiryStr)
		return nil, err
	}

	processorLogger, err := config.NewLogger()
	if err != nil {
		logger.Errorf("failed to get logger: %v", err)
		return nil, err
	}

	dataRetentionDays, err := config.GetInt("metamorph.db.cleanData.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	metamorphProcessor, err := metamorph.NewProcessor(
		s,
		pm,
		btx,
		metamorph.WithCacheExpiryTime(mapExpiry),
		metamorph.WithProcessorLogger(processorLogger),
		metamorph.WithLogFilePath(viper.GetString("metamorph.log.file")),
		metamorph.WithDataRetentionPeriod(time.Duration(dataRetentionDays)*24*time.Hour),
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
	ticker := time.NewTimer(unminedTxsPeriod)
	stopUnminedProcessor := make(chan bool)

	go func() {
		for {
			select {
			case <-stopUnminedProcessor:
				return
			case <-ticker.C:
				metamorphProcessor.LoadUnmined()
			}
		}
	}()

	// create a channel to receive mined block messages from the block tx service
	blockChan := make(chan *blocktx_api.Block)
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
				processBlock(logger, btx, metamorphProcessor, s, &blocktx_api.BlockAndSource{
					Hash:   block.GetHash(),
					Source: source,
				})
			}

			// check whether we have already processed the previous block
			blockHash, _ := chainhash.NewHash(block.GetPreviousHash())

			processedAt, err = s.GetBlockProcessed(context.Background(), blockHash)
			if err != nil {
				logger.Errorf("Could not get previous block processed status: %v", err)
				continue
			}
			if processedAt == nil || processedAt.IsZero() {
				// get the full previous block from block tx
				var previousBlock *blocktx_api.Block
				pHash, err := chainhash.NewHash(block.GetPreviousHash())
				if err != nil {
					logger.Errorf("Could not get previous block hash: %v", err)
					continue
				}

				previousBlock, err = btx.GetBlock(context.Background(), pHash)
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

	btx.Start(blockChan)

	metamorphLogger, err := config.NewLogger()
	if err != nil {
		logger.Errorf("failed to get new logger: %v", err)
		return nil, err
	}
	opts := []metamorph.ServerOption{
		metamorph.WithLogger(metamorphLogger),
	}

	if viper.GetBool("metamorph.checkUtxos") {
		peerRpcPassword := viper.GetString("peerRpc.password")
		if peerRpcPassword == "" {
			return nil, errors.Errorf("setting peerRpc.password not found")
		}

		peerRpcUser := viper.GetString("peerRpc.user")
		if peerRpcUser == "" {
			return nil, errors.Errorf("setting peerRpc.user not found")
		}

		peerRpcHost := viper.GetString("peerRpc.host")
		if peerRpcHost == "" {
			return nil, errors.Errorf("setting peerRpc.host not found")
		}

		peerRpcPort := viper.GetInt("peerRpc.port")
		if peerRpcPort == 0 {
			return nil, errors.Errorf("setting peerRpc.port not found")
		}

		rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpcUser, peerRpcPassword, peerRpcHost, peerRpcPort))
		if err != nil {
			return nil, errors.Errorf("failed to parse rpc URL: %v", err)
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
		logger.Infof("Shutting down metamorph")

		stopUnminedProcessor <- true
		metamorphProcessor.Shutdown()
		err = s.Close(context.Background())
		if err != nil {
			logger.Errorf("Could not close store: %v", err)
		}
	}, nil
}

func NewStore(dbMode string, folder string) (s store.MetamorphStore, err error) {
	switch dbMode {
	case DbModePostgres:
		var hostname string
		hostname, err = os.Hostname()
		if err != nil {
			return nil, err
		}
		dbHost, err := config.GetString("metamorph.db.postgres.host")
		if err != nil {
			return nil, err
		}
		dbPort, err := config.GetInt("metamorph.db.postgres.port")
		if err != nil {
			return nil, err
		}
		dbName, err := config.GetString("metamorph.db.postgres.name")
		if err != nil {
			return nil, err
		}
		dbUser, err := config.GetString("metamorph.db.postgres.user")
		if err != nil {
			return nil, err
		}
		dbPassword, err := config.GetString("metamorph.db.postgres.password")
		if err != nil {
			return nil, err
		}
		sslMode, err := config.GetString("metamorph.db.postgres.sslMode")
		if err != nil {
			return nil, err
		}
		idleConns, err := config.GetInt("metamorph.db.postgres.maxIdleConns")
		if err != nil {
			return nil, err
		}
		maxOpenConns, err := config.GetInt("metamorph.db.postgres.maxOpenConns")
		if err != nil {
			return nil, err
		}

		dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=%s", dbUser, dbPassword, dbName, dbHost, dbPort, sslMode)
		s, err = postgresql.New(dbInfo, hostname, idleConns, maxOpenConns)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	case DbModeSQLite:
		s, err = sqlite.New(false, folder)
		if err != nil {
			return nil, err
		}
	case DbModeSQLiteM:
		s, err = sqlite.New(true, "")
		if err != nil {
			return nil, err
		}
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

		dataRetentionDays, err := config.GetInt("metamorph.db.cleanData.recordRetentionDays")
		if err != nil {
			return nil, err
		}

		tableNameSuffix := viper.GetString("metamorph.db.dynamoDB.tableNameSuffix")

		s, err = dynamodb.New(
			awsdynamodb.NewFromConfig(cfg),
			hostname,
			time.Duration(dataRetentionDays)*24*time.Hour,
			tableNameSuffix,
		)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbMode)
	}

	return s, err
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
		logger.Errorf("Could not get mined transactions for block %x: %v", bt.ReverseBytes(blockAndSource.GetHash()), err)
		return
	}

	logger.Infof("Incoming BLOCK %x", bt.ReverseBytes(mt.GetBlock().GetHash()))

	for _, tx := range mt.GetTransactions() {
		logger.Debugf("Received MINED message from BlockTX for transaction %x", bt.ReverseBytes(tx.GetHash()))

		hash, _ := chainhash.NewHash(tx.GetHash())
		blockHash, _ := chainhash.NewHash(mt.GetBlock().GetHash())

		_, err = p.SendStatusMinedForTransaction(hash, blockHash, mt.GetBlock().GetHeight())
		if err != nil {
			logger.Errorf("Could not send mined status for transaction %x: %v", bt.ReverseBytes(tx.GetHash()), err)
			return
		}
	}

	logger.Infof("Marked %d transactions as MINED", len(mt.GetTransactions()))

	hash, _ := chainhash.NewHash(blockAndSource.GetHash())

	err = s.SetBlockProcessed(context.Background(), hash)
	if err != nil {
		logger.Errorf("Could not set block processed status: %v", err)
	}
}
