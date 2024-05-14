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
	"runtime/debug"
	"strconv"
	"time"

	cfg "github.com/bitcoin-sv/arc/internal/config"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/async"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/metamorph/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/nats_mq"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/libsv/go-p2p"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils/safemap"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	DbModePostgres = "postgres"
)

func StartMetamorph(logger *slog.Logger) (func(), error) {
	logger = logger.With(slog.String("service", "mtm"))

	dbMode, err := cfg.GetString("metamorph.db.mode")
	if dbMode == "" {
		return nil, errors.New("metamorph.db.mode not found in config")
	}

	s, err := NewMetamorphStore(dbMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create metamorph store: %v", err)
	}

	pm, statusMessageCh, err := initPeerManager(logger.With(slog.String("module", "mtm-peer-handler")), s)
	if err != nil {
		return nil, err
	}

	mapExpiryStr, err := cfg.GetString("metamorph.processorCacheExpiryTime")
	if err != nil {
		return nil, err
	}

	mapExpiry, err := time.ParseDuration(mapExpiryStr)
	if err != nil {
		return nil, fmt.Errorf("invalid metamorph.processorCacheExpiryTime: %s", mapExpiryStr)
	}

	seenOnNetworkOlderThan, err := cfg.GetDuration("metamorph.checkSeenOnNetworkOlderThan")
	if err != nil {
		return nil, err
	}

	checkSeenOnNetworkPeriod, err := cfg.GetDuration("metamorph.checkSeenOnNetworkPeriod")
	if err != nil {
		return nil, err
	}

	statsNotSeenTimeLimit, err := cfg.GetDuration("metamorph.stats.notSeenTimeLimit")
	if err != nil {
		return nil, err
	}

	statsNotMinedTimeLimit, err := cfg.GetDuration("metamorph.stats.notMinedTimeLimit")
	if err != nil {
		return nil, err
	}

	natsURL, err := cfg.GetString("queueURL")
	if err != nil {
		return nil, err
	}

	maxRetries, err := cfg.GetInt("metamorph.maxRetries")
	if err != nil {
		return nil, err
	}

	// The tx channel needs the capacity so that it could potentially buffer up to a certain nr of transactions per second
	const targetTps = 6000
	const avgMinPerBlock = 10
	const secPerMin = 60

	maxBatchSize, err := cfg.GetInt("blocktx.mq.txsMinedMaxBatchSize")
	if err != nil {
		return nil, err
	}

	processStatusUpdateInterval, err := cfg.GetDuration("metamorph.processStatusUpdateInterval")
	if err != nil {
		return nil, err
	}

	// maximum amount of messages that could be coming from a single block
	capacityRequired := int(float64(targetTps*avgMinPerBlock*secPerMin) / float64(maxBatchSize))
	minedTxsChan := make(chan *blocktx_api.TransactionBlocks, capacityRequired)

	natsClient, err := nats_mq.NewNatsClient(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", natsURL, err)
	}

	mqClient := async.NewNatsMQClient(natsClient, minedTxsChan, logger)

	err = mqClient.SubscribeMinedTxs()
	if err != nil {
		return nil, err
	}

	metamorphProcessor, err := metamorph.NewProcessor(
		s,
		pm,
		metamorph.WithCacheExpiryTime(mapExpiry),
		metamorph.WithSeenOnNetworkTxTimeUntil(seenOnNetworkOlderThan),
		metamorph.WithSeenOnNetworkTxTime(checkSeenOnNetworkPeriod),
		metamorph.WithProcessorLogger(logger.With(slog.String("module", "mtm-proc"))),
		metamorph.WithMessageQueueClient(mqClient),
		metamorph.WithMinedTxsChan(minedTxsChan),
		metamorph.WithProcessStatusUpdatesInterval(processStatusUpdateInterval),
		metamorph.WithCallbackSender(metamorph.NewCallbacker(&http.Client{Timeout: 5 * time.Second})),
		metamorph.WithStatTimeLimits(statsNotSeenTimeLimit, statsNotMinedTimeLimit),
		metamorph.WithMaxRetries(maxRetries),
	)
	if err != nil {
		return nil, err
	}

	metamorphProcessor.StartLockTransactions()
	time.Sleep(200 * time.Millisecond) // wait a short time so that process expired transactions will start shortly after lock transactions go routine

	metamorphProcessor.StartProcessExpiredTransactions()
	metamorphProcessor.StartRequestingSeenOnNetworkTxs()
	metamorphProcessor.StartProcessStatusUpdatesInStorage()
	metamorphProcessor.StartProcessMinedCallbacks()
	err = metamorphProcessor.StartCollectStats()
	if err != nil {
		return nil, fmt.Errorf("failed to start collecting stats: %v", err)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Recovered from panic", "panic", r, slog.String("stacktrace", string(debug.Stack())))
			}
		}()

		for message := range statusMessageCh {
			err = metamorphProcessor.SendStatusForTransaction(message.Hash, message.Status, message.Peer, message.Err)
			if err != nil {
				logger.Error("Could not send status for transaction", slog.String("hash", message.Hash.String()), slog.String("err", err.Error()))
			}
		}
	}()

	optsServer := []metamorph.ServerOption{
		metamorph.WithLogger(logger.With(slog.String("module", "mtm-server"))),
	}

	if viper.GetBool("metamorph.checkUtxos") {
		peerRpcPassword, err := cfg.GetString("peerRpc.password")
		if err != nil {
			return nil, err
		}

		peerRpcUser, err := cfg.GetString("peerRpc.user")
		if err != nil {
			return nil, err
		}

		peerRpcHost, err := cfg.GetString("peerRpc.host")
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

	server := metamorph.NewServer(s, metamorphProcessor, optsServer...)

	metamorphGRPCListenAddress, err := cfg.GetString("metamorph.listenAddr")
	if err != nil {
		return nil, err
	}

	grpcMessageSize, err := cfg.GetInt("grpcMessageSize")
	if err != nil {
		return nil, err
	}

	prometheusEndpoint := viper.GetString("prometheusEndpoint")

	err = server.StartGRPCServer(metamorphGRPCListenAddress, grpcMessageSize, prometheusEndpoint, logger)
	if err != nil {
		return nil, fmt.Errorf("GRPCServer failed: %v", err)
	}

	peerSettings, err := cfg.GetPeerSettings()
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

		zmq := metamorph.NewZMQ(zmqURL, statusMessageCh, logger)
		zmqCollector.Set(zmqURL.Host, zmq.GetStats())

		port, err := strconv.Atoi(zmqURL.Port())
		if err != nil {
			return nil, fmt.Errorf("failed to parse port from peer settings: %v", err)
		}

		logger.Info("Listening to ZMQ", slog.String("host", zmqURL.Hostname()), slog.Int("port", port))

		zmqLogger := logrus.New()
		zmqLogger.SetFormatter(&logrus.JSONFormatter{})
		err = zmq.Start(bitcoin.NewZMQ(zmqURL.Hostname(), port, zmqLogger))
		if err != nil {
			return nil, fmt.Errorf("failed to start ZMQ: %v", err)
		}
	}

	// pass all the started peers to the collector
	_ = metamorph.NewZMQCollector(zmqCollector)

	healthServer, err := StartHealthServerMetamorph(server, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	return func() {
		logger.Info("Shutting down metamorph")

		err = mqClient.Shutdown()
		if err != nil {
			logger.Error("failed to shutdown mqClient", slog.String("err", err.Error()))
		}

		metamorphProcessor.Shutdown()
		err = s.Close(context.Background())
		if err != nil {
			logger.Error("Could not close store", slog.String("err", err.Error()))
		}

		server.Shutdown()

		healthServer.Stop()
	}, nil
}

func StartHealthServerMetamorph(serv *metamorph.Server, logger *slog.Logger) (*grpc.Server, error) {
	gs := grpc.NewServer()

	grpc_health_v1.RegisterHealthServer(gs, serv) // registration
	// register your own services
	reflection.Register(gs)

	address, err := cfg.GetString("metamorph.healthServerDialAddr") //"localhost:8005"
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Recovered from panic", "panic", r, slog.String("stacktrace", string(debug.Stack())))
			}
		}()

		logger.Info("GRPC health server listening", slog.String("address", address))
		err = gs.Serve(listener)
		if err != nil {
			logger.Error("GRPC health server failed to serve", slog.String("err", err.Error()))
		}
	}()

	return gs, nil
}

func NewMetamorphStore(dbMode string) (s store.MetamorphStore, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	switch dbMode {
	case DbModePostgres:
		dbHost, err := cfg.GetString("metamorph.db.postgres.host")
		if err != nil {
			return nil, err
		}
		dbPort, err := cfg.GetInt("metamorph.db.postgres.port")
		if err != nil {
			return nil, err
		}
		dbName, err := cfg.GetString("metamorph.db.postgres.name")
		if err != nil {
			return nil, err
		}
		dbUser, err := cfg.GetString("metamorph.db.postgres.user")
		if err != nil {
			return nil, err
		}
		dbPassword, err := cfg.GetString("metamorph.db.postgres.password")
		if err != nil {
			return nil, err
		}
		sslMode, err := cfg.GetString("metamorph.db.postgres.sslMode")
		if err != nil {
			return nil, err
		}
		idleConns, err := cfg.GetInt("metamorph.db.postgres.maxIdleConns")
		if err != nil {
			return nil, err
		}
		maxOpenConns, err := cfg.GetInt("metamorph.db.postgres.maxOpenConns")
		if err != nil {
			return nil, err
		}

		dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=%s", dbUser, dbPassword, dbName, dbHost, dbPort, sslMode)
		s, err = postgresql.New(dbInfo, hostname, idleConns, maxOpenConns)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbMode)
	}

	return s, err
}

func initPeerManager(logger *slog.Logger, s store.MetamorphStore) (p2p.PeerManagerI, chan *metamorph.PeerTxMessage, error) {

	network, err := cfg.GetNetwork()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get network: %v", err)
	}

	logger.Info("Assuming bitcoin network", "network", network)

	messageCh := make(chan *metamorph.PeerTxMessage)
	pm := p2p.NewPeerManager(logger, network)

	peerHandler := metamorph.NewPeerHandler(s, messageCh)

	peerSettings, err := cfg.GetPeerSettings()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting peer settings: %v", err)
	}

	opts := make([]p2p.PeerOptions, 0)
	if version.Version != "" {
		opts = append(opts, p2p.WithUserAgent("ARC", version.Version))
	}

	opts = append(opts, p2p.WithRetryReadWriteMessageInterval(5*time.Second))

	for _, peerSetting := range peerSettings {
		peerUrl, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, nil, fmt.Errorf("error getting peer url: %v", err)
		}

		var peer *p2p.Peer
		peer, err = p2p.NewPeer(logger, peerUrl, peerHandler, network, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating peer %s: %v", peerUrl, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			return nil, nil, fmt.Errorf("error adding peer %s: %v", peerUrl, err)
		}
	}

	return pm, messageCh, nil
}
