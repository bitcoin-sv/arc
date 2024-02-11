package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/async/nats_mq"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/metamorph/store/badger"
	"github.com/bitcoin-sv/arc/metamorph/store/dynamodb"
	"github.com/bitcoin-sv/arc/metamorph/store/postgresql"
	"github.com/bitcoin-sv/arc/metamorph/store/sqlite"
	"github.com/libsv/go-p2p"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils/safemap"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	DbModeBadger   = "badger"
	DbModeDynamoDB = "dynamodb"
	DbModePostgres = "postgres"
	DbModeSQLiteM  = "sqlite_memory"
	DbModeSQLite   = "sqlite"
)

func StartMetamorph(logger *slog.Logger) (func(), error) {
	logger = logger.With(slog.String("service", "mtm"))

	dbMode, err := config.GetString("metamorph.db.mode")
	if dbMode == "" {
		return nil, errors.New("metamorph.db.mode not found in config")
	}

	s, err := NewStore(dbMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create metamorph store: %v", err)
	}

	blocktxAddress, err := config.GetString("blocktx.dialAddr")
	if err != nil {
		return nil, err
	}

	conn, err := blocktx.DialGRPC(blocktxAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	btx := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(conn))

	metamorphGRPCListenAddress, err := config.GetString("metamorph.listenAddr")
	if err != nil {
		return nil, err
	}

	pm, statusMessageCh, err := initPeerManager(logger.With(slog.String("module", "mtm-peer-handler")), s)
	if err != nil {
		return nil, err
	}

	mapExpiryStr, err := config.GetString("metamorph.processorCacheExpiryTime")
	if err != nil {
		return nil, err
	}

	mapExpiry, err := time.ParseDuration(mapExpiryStr)
	if err != nil {
		return nil, fmt.Errorf("invalid metamorph.processorCacheExpiryTime: %s", mapExpiryStr)
	}

	dataRetentionDays, err := config.GetInt("metamorph.db.cleanData.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	maxMonitoredTxs, err := config.GetInt64("metamorph.maxMonitoredTxs")
	if err != nil {
		return nil, err
	}

	natsURL, err := config.GetString("queueURL")
	if err != nil {
		return nil, err
	}

	minedTxsChan := make(chan *blocktx_api.TransactionBlocks, 400)

	mqClient, err := nats_mq.NewNatsMQClient(minedTxsChan, logger, natsURL)
	if err != nil {
		return nil, err
	}

	err = mqClient.SubscribeMinedTxs()
	if err != nil {
		return nil, err
	}

	metamorphProcessor, err := metamorph.NewProcessor(
		s,
		pm,
		btx,
		metamorph.WithCacheExpiryTime(mapExpiry),
		metamorph.WithProcessorLogger(logger.With(slog.String("module", "mtm-proc"))),
		metamorph.WithDataRetentionPeriod(time.Duration(dataRetentionDays)*24*time.Hour),
		metamorph.WithMaxMonitoredTxs(maxMonitoredTxs),
		metamorph.WithPublisher(publisher),
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

	unminedTxsPeriod, err := config.GetDuration("metamorph.loadUnminedPeriod")
	if err != nil {
		return nil, err
	}

	ticker := time.NewTimer(unminedTxsPeriod)
	stopUnminedProcessor := make(chan struct{})

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

	optsServer := []metamorph.ServerOption{
		metamorph.WithLogger(logger.With(slog.String("module", "mtm-server"))),
	}

	if viper.GetBool("metamorph.checkUtxos") {
		peerRpcPassword, err := config.GetString("peerRpc.password")
		if err != nil {
			return nil, err
		}

		peerRpcUser, err := config.GetString("peerRpc.user")
		if err != nil {
			return nil, err
		}

		peerRpcHost, err := config.GetString("peerRpc.host")
		if err != nil {
			return nil, err
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

		optsServer = append(optsServer, metamorph.WithForceCheckUtxos(node))
	}

	btxTimeout := viper.GetDuration("metamorph.blocktxTimeout")
	if btxTimeout > 0 {
		optsServer = append(optsServer, metamorph.WithBlocktxTimeout(btxTimeout))
	}

	serv := metamorph.NewServer(s, metamorphProcessor, btx, optsServer...)

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

	peerSettings, err := config.GetPeerSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get peer settings: %v", err)
	}

	zmqCollector := safemap.New[string, *metamorph.ZMQStats]()

	for i, peerSetting := range peerSettings {
		zmqURL, err := peerSetting.GetZMQUrl()
		if err != nil {
			logger.Warn("failed to get zmq URL for peer", slog.Int("index", i), slog.String("err", err.Error()))
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

	go func() {
		err = StartHealthServer(serv)
		if err != nil {
			logger.Error("failed to start health server", slog.String("err", err.Error()))
		}
	}()

	return func() {
		logger.Info("Shutting down metamorph")
		err = mqClient.Shutdown()
		if err != nil {
			logger.Error("failed to shutdown mqClient", slog.String("err", err.Error()))
		}
		stopUnminedProcessor <- struct{}{}
		metamorphProcessor.Shutdown()
		err = s.Close(context.Background())
		if err != nil {
			logger.Error("Could not close store", slog.String("err", err.Error()))
		}
	}, nil
}

func StartHealthServer(serv *metamorph.Server) error {
	gs := grpc.NewServer()
	defer gs.Stop()

	grpc_health_v1.RegisterHealthServer(gs, serv) // registration
	// register your own services
	reflection.Register(gs)

	address, err := config.GetString("metamorph.healthServerDialAddr") //"localhost:8005"
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	err = gs.Serve(listener)
	if err != nil {
		return err
	}

	return nil
}

func getDataFolder() (string, error) {
	folder, err := config.GetString("dataFolder")
	if folder == "" {
		return "", fmt.Errorf("dataFolder not found in config: %v", err)
	}

	if err := os.MkdirAll(folder, 0750); err != nil {
		return "", fmt.Errorf("failed to create data folder %s: %+v", folder, err)
	}

	return folder, nil
}

func NewStore(dbMode string) (s store.MetamorphStore, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	switch dbMode {
	case DbModePostgres:
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
		folder, err := getDataFolder()
		if err != nil {
			return nil, err
		}

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
		folder, err := getDataFolder()
		if err != nil {
			return nil, err
		}

		s, err = badger.New(path.Join(folder, "metamorph"))
		if err != nil {
			return nil, err
		}
	case DbModeDynamoDB:
		ctx := context.Background()
		cfg, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithEC2IMDSRegion())
		if err != nil {
			return nil, err
		}

		dataRetentionDays, err := config.GetInt("metamorph.db.cleanData.recordRetentionDays")
		if err != nil {
			return nil, err
		}

		tableNameSuffix, err := config.GetString("metamorph.db.dynamoDB.tableNameSuffix")
		if err != nil {
			return nil, err
		}

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

func initPeerManager(logger *slog.Logger, s store.MetamorphStore) (p2p.PeerManagerI, chan *metamorph.PeerTxMessage, error) {

	network, err := config.GetNetwork()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get network: %v", err)
	}

	logger.Info("Assuming bitcoin network", "network", network)

	messageCh := make(chan *metamorph.PeerTxMessage)
	pm := p2p.NewPeerManager(logger, network)

	peerHandler := metamorph.NewPeerHandler(s, messageCh)

	peerSettings, err := config.GetPeerSettings()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting peer settings: %v", err)
	}

	for _, peerSetting := range peerSettings {
		peerUrl, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, nil, fmt.Errorf("error getting peer url: %v", err)
		}

		var peer *p2p.Peer
		peer, err = p2p.NewPeer(logger, peerUrl, peerHandler, network)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating peer %s: %v", peerUrl, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			return nil, nil, fmt.Errorf("error adding peer %s: %v", peerUrl, err)
		}
	}

	return pm, messageCh, nil
}
