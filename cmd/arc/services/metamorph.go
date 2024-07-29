package cmd

import (
	"context"
	"fmt"
	"github.com/bitcoin-sv/arc/internal/nats_mq"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/async"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/metamorph/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/libsv/go-p2p"
	"github.com/ordishs/go-bitcoin"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	DbModePostgres = "postgres"
	chanBufferSize = 4000
)

func StartMetamorph(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	logger = logger.With(slog.String("service", "mtm"))
	mtmConfig := arcConfig.Metamorph

	metamorphStore, err := NewMetamorphStore(mtmConfig.Db)
	if err != nil {
		return nil, fmt.Errorf("failed to create metamorph store: %v", err)
	}

	pm, peerHandler, statusMessageCh, err := initPeerManager(logger, metamorphStore, arcConfig)
	if err != nil {
		return nil, err
	}

	// maximum amount of messages that could be coming from a single block
	minedTxsChan := make(chan *blocktx_api.TransactionBlock, chanBufferSize)
	submittedTxsChan := make(chan *metamorph_api.TransactionRequest, chanBufferSize)

	var mqClient metamorph.MessageQueueClient
	natsClient, err := nats_mq.NewNatsClient(arcConfig.MessageQueue.URL, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
	}

	if arcConfig.MessageQueue.EnableStreaming {
		ctx := context.Background()
		mqClient, err = async.NewJetStreamClient(ctx, natsClient, logger, arcConfig.MessageQueue.URL, async.WithConsumer())
		if err != nil {
			return nil, fmt.Errorf("failed to create nats client: %v", err)
		}
	} else {
		mqClient = async.NewNatsMQClient(natsClient, async.WithLogger(logger))
	}

	callbacker, err := metamorph.NewCallbacker(&http.Client{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}

	processorOpts := []metamorph.Option{
		metamorph.WithCacheExpiryTime(mtmConfig.ProcessorCacheExpiryTime),
		metamorph.WithSeenOnNetworkTxTimeUntil(mtmConfig.CheckSeenOnNetworkOlderThan),
		metamorph.WithSeenOnNetworkTxTime(mtmConfig.CheckSeenOnNetworkPeriod),
		metamorph.WithProcessorLogger(logger.With(slog.String("module", "mtm-proc"))),
		metamorph.WithMessageQueueClient(mqClient),
		metamorph.WithMinedTxsChan(minedTxsChan),
		metamorph.WithSubmittedTxsChan(submittedTxsChan),
		metamorph.WithProcessStatusUpdatesInterval(mtmConfig.ProcessStatusUpdateInterval),
		metamorph.WithCallbackSender(callbacker),
		metamorph.WithStatTimeLimits(mtmConfig.Stats.NotSeenTimeLimit, mtmConfig.Stats.NotMinedTimeLimit),
		metamorph.WithMaxRetries(mtmConfig.MaxRetries),
		metamorph.WithMinimumHealthyConnections(mtmConfig.Health.MinimumHealthyConnections),
	}

	processor, err := metamorph.NewProcessor(
		metamorphStore,
		pm,
		statusMessageCh,
		processorOpts...,
	)
	if err != nil {
		return nil, err
	}
	err = processor.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start metamorph processor: %v", err)
	}

	optsServer := []metamorph.ServerOption{
		metamorph.WithLogger(logger.With(slog.String("module", "mtm-server"))),
	}

	if mtmConfig.CheckUtxos {
		peerRpc := arcConfig.PeerRpc

		rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpc.User, peerRpc.Password, peerRpc.Host, peerRpc.Port))
		if err != nil {
			return nil, fmt.Errorf("failed to parse rpc URL: %v", err)
		}

		node, err := bitcoin.NewFromURL(rpcURL, false)
		if err != nil {
			return nil, err
		}

		optsServer = append(optsServer, metamorph.WithForceCheckUtxos(node))
	}

	server := metamorph.NewServer(metamorphStore, processor, optsServer...)

	err = server.StartGRPCServer(mtmConfig.ListenAddr, arcConfig.GrpcMessageSize, arcConfig.PrometheusEndpoint, logger)
	if err != nil {
		return nil, fmt.Errorf("GRPCServer failed: %v", err)
	}

	for i, peerSetting := range arcConfig.Peers {
		zmqURL, err := peerSetting.GetZMQUrl()
		if err != nil {
			logger.Warn("failed to get zmq URL for peer", slog.Int("index", i), slog.String("err", err.Error()))
			continue
		}

		zmq := metamorph.NewZMQ(zmqURL, statusMessageCh, logger)

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

	healthServer, err := StartHealthServerMetamorph(server, mtmConfig.Health, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	return func() {
		logger.Info("Shutting down metamorph")

		server.Shutdown()

		mqClient.Shutdown()
		if err != nil {
			logger.Error("failed to shutdown mqClient", slog.String("err", err.Error()))
		}

		peerHandler.Shutdown()

		err = metamorphStore.Close(context.Background())
		if err != nil {
			logger.Error("Could not close store", slog.String("err", err.Error()))
		}

		healthServer.Stop()
	}, nil
}

func StartHealthServerMetamorph(serv *metamorph.Server, healthConfig *config.HealthConfig, logger *slog.Logger) (*grpc.Server, error) {
	gs := grpc.NewServer()

	grpc_health_v1.RegisterHealthServer(gs, serv) // registration
	// register your own services
	reflection.Register(gs)

	listener, err := net.Listen("tcp", healthConfig.SeverDialAddr)
	if err != nil {
		return nil, err
	}

	go func() {
		logger.Info("GRPC health server listening", slog.String("address", healthConfig.SeverDialAddr))
		err = gs.Serve(listener)
		if err != nil {
			logger.Error("GRPC health server failed to serve", slog.String("err", err.Error()))
		}
	}()

	return gs, nil
}

func NewMetamorphStore(dbConfig *config.DbConfig) (s store.MetamorphStore, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	switch dbConfig.Mode {
	case DbModePostgres:
		postgres := dbConfig.Postgres

		dbInfo := fmt.Sprintf(
			"user=%s password=%s dbname=%s host=%s port=%d sslmode=%s",
			postgres.User, postgres.Password, postgres.Name, postgres.Host, postgres.Port, postgres.SslMode,
		)
		s, err = postgresql.New(dbInfo, hostname, postgres.MaxIdleConns, postgres.MaxOpenConns)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbConfig.Mode)
	}

	return s, err
}

func initPeerManager(logger *slog.Logger, s store.MetamorphStore, arcConfig *config.ArcConfig) (p2p.PeerManagerI, *metamorph.PeerHandler, chan *metamorph.PeerTxMessage, error) {
	logger = logger.With(slog.String("module", "mtm-peer-handler"))

	network, err := config.GetNetwork(arcConfig.Network)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get network: %v", err)
	}

	logger.Info("Assuming bitcoin network", "network", network)

	messageCh := make(chan *metamorph.PeerTxMessage)
	var pmOpts []p2p.PeerManagerOptions
	if arcConfig.Metamorph.MonitorPeers {
		pmOpts = append(pmOpts, p2p.WithRestartUnhealthyPeers())
	}

	pm := p2p.NewPeerManager(logger, network, pmOpts...)

	peerHandler := metamorph.NewPeerHandler(s, messageCh)

	peerOpts := []p2p.PeerOptions{
		p2p.WithRetryReadWriteMessageInterval(5 * time.Second),
		p2p.WithPingInterval(30*time.Second, 1*time.Minute),
	}
	if version.Version != "" {
		peerOpts = append(peerOpts, p2p.WithUserAgent("ARC", version.Version))
	}

	for _, peerSetting := range arcConfig.Peers {
		peerUrl, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error getting peer url: %v", err)
		}

		var peer *p2p.Peer
		peer, err = p2p.NewPeer(logger, peerUrl, peerHandler, network, peerOpts...)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error creating peer %s: %v", peerUrl, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			return nil, nil, nil, fmt.Errorf("error adding peer %s: %v", peerUrl, err)
		}
	}

	return pm, peerHandler, messageCh, nil
}
