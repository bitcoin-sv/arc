package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/libsv/go-p2p"
	"google.golang.org/grpc"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/tracing"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/metamorph/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/version"
)

const (
	DbModePostgres = "postgres"
	chanBufferSize = 4000
)

func StartMetamorph(logger *slog.Logger, arcConfig *config.ArcConfig, cacheStore cache.Store) (func(), error) {
	logger = logger.With(slog.String("service", "mtm"))
	logger.Info("Starting")

	mtmConfig := arcConfig.Metamorph

	var (
		metamorphStore  store.MetamorphStore
		peerHandler     *metamorph.PeerHandler
		pm              metamorph.PeerManager
		statusMessageCh chan *metamorph.TxStatusMessage
		mqClient        metamorph.MessageQueue
		processor       *metamorph.Processor
		server          *metamorph.Server
		healthServer    *grpc_opts.GrpcServer

		err error
	)

	shutdownFns := make([]func(), 0)

	optsServer := make([]metamorph.ServerOption, 0)
	processorOpts := make([]metamorph.Option, 0)
	callbackerOpts := make([]callbacker.Option, 0)

	if arcConfig.IsTracingEnabled() {
		cleanup, err := tracing.Enable(logger, "metamorph", arcConfig.Tracing)
		if err != nil {
			logger.Error("failed to enable tracing", slog.String("err", err.Error()))
		} else {
			shutdownFns = append(shutdownFns, cleanup)
		}

		attributes := arcConfig.Tracing.KeyValueAttributes
		hostname, err := os.Hostname()
		if err == nil {
			hostnameAttr := attribute.String("hostname", hostname)
			attributes = append(attributes, hostnameAttr)
		}

		optsServer = append(optsServer, metamorph.WithServerTracer(attributes...))
		callbackerOpts = append(callbackerOpts, callbacker.WithTracerCallbacker(attributes...))
		processorOpts = append(processorOpts, metamorph.WithTracerProcessor(attributes...))
	}

	stopFn := func() {
		logger.Info("Shutting down metamorph")
		disposeMtm(logger, server, processor, peerHandler, mqClient, metamorphStore, healthServer, shutdownFns)
		logger.Info("Shutdown complete")
	}

	metamorphStore, err = NewMetamorphStore(mtmConfig.Db, arcConfig.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to create metamorph store: %v", err)
	}

	pm, peerHandler, statusMessageCh, err = initPeerManager(logger, metamorphStore, arcConfig)
	if err != nil {
		stopFn()
		return nil, err
	}

	// maximum amount of messages that could be coming from a single block
	minedTxsChan := make(chan *blocktx_api.TransactionBlock, chanBufferSize)
	submittedTxsChan := make(chan *metamorph_api.TransactionRequest, chanBufferSize)

	natsClient, err := nats_connection.New(arcConfig.MessageQueue.URL, logger)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
	}

	if arcConfig.MessageQueue.Streaming.Enabled {
		opts := []nats_jetstream.Option{
			nats_jetstream.WithSubscribedWorkQueuePolicy(metamorph.MinedTxsTopic, metamorph.SubmitTxTopic),
			nats_jetstream.WithWorkQueuePolicy(metamorph.RegisterTxTopic),
			nats_jetstream.WithInterestPolicy(metamorph.CallbackTopic),
		}
		if arcConfig.MessageQueue.Streaming.FileStorage {
			opts = append(opts, nats_jetstream.WithFileStorage())
		}

		if arcConfig.Tracing.Enabled {
			opts = append(opts, nats_jetstream.WithTracer(arcConfig.Tracing.KeyValueAttributes...))
		}

		mqClient, err = nats_jetstream.New(natsClient, logger,
			opts...,
		)
		if err != nil {
			stopFn()
			return nil, fmt.Errorf("failed to create nats client: %v", err)
		}
	} else {
		opts := []nats_core.Option{nats_core.WithLogger(logger)}
		if arcConfig.Tracing.Enabled {
			opts = append(opts, nats_core.WithTracer(arcConfig.Tracing.KeyValueAttributes...))
		}
		mqClient = nats_core.New(natsClient, opts...)
	}

	procLogger := logger.With(slog.String("module", "mtm-proc"))

	callbackerConn, err := initGrpcCallbackerConn(arcConfig.Callbacker.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, arcConfig.Tracing)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to create callbacker client: %v", err)
	}

	callbacker := callbacker.NewGrpcCallbacker(callbackerConn, procLogger, callbackerOpts...)

	processorOpts = append(processorOpts, metamorph.WithCacheExpiryTime(mtmConfig.ProcessorCacheExpiryTime),
		metamorph.WithProcessExpiredTxsInterval(mtmConfig.UnseenTransactionRebroadcastingInterval),
		metamorph.WithRecheckSeenUntilAgo(mtmConfig.RecheckSeen.UntilAgo),
		metamorph.WithRecheckSeenFromAgo(mtmConfig.RecheckSeen.FromAgo),
		metamorph.WithProcessorLogger(procLogger),
		metamorph.WithMessageQueueClient(mqClient),
		metamorph.WithMinedTxsChan(minedTxsChan),
		metamorph.WithSubmittedTxsChan(submittedTxsChan),
		metamorph.WithProcessStatusUpdatesInterval(mtmConfig.ProcessStatusUpdateInterval),
		metamorph.WithCallbackSender(callbacker),
		metamorph.WithStatTimeLimits(mtmConfig.Stats.NotSeenTimeLimit, mtmConfig.Stats.NotFinalTimeLimit),
		metamorph.WithMaxRetries(mtmConfig.MaxRetries),
		metamorph.WithMinimumHealthyConnections(mtmConfig.Health.MinimumHealthyConnections))

	processor, err = metamorph.NewProcessor(
		metamorphStore,
		cacheStore,
		pm,
		statusMessageCh,
		processorOpts...,
	)
	if err != nil {
		stopFn()
		return nil, err
	}
	err = processor.Start(arcConfig.Prometheus.IsEnabled())
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start metamorph processor: %v", err)
	}

	server, err = metamorph.NewServer(arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, logger,
		metamorphStore, processor, arcConfig.Tracing, optsServer...)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("create GRPCServer failed: %v", err)
	}
	err = server.ListenAndServe(mtmConfig.ListenAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("serve GRPC server failed: %v", err)
	}

	for i, peerSetting := range arcConfig.Broadcasting.Unicast.Peers {
		zmqURL, err := peerSetting.GetZMQUrl()
		if err != nil {
			logger.Warn("failed to get zmq URL for peer", slog.Int("index", i), slog.String("err", err.Error()))
			continue
		}

		if zmqURL == nil {
			continue
		}

		zmqHandler := metamorph.NewZMQHandler(context.Background(), zmqURL, logger)
		zmq, err := metamorph.NewZMQ(zmqURL, statusMessageCh, zmqHandler, logger)
		if err != nil {
			stopFn()
			return nil, fmt.Errorf("failed to create ZMQ: %v", err)
		}
		logger.Info("Listening to ZMQ", slog.String("host", zmqURL.Hostname()), slog.String("port", zmqURL.Port()))

		cleanup, err := zmq.Start()
		shutdownFns = append(shutdownFns, cleanup)
		if err != nil {
			stopFn()
			return nil, fmt.Errorf("failed to start ZMQ: %v", err)
		}
	}

	healthServer, err = grpc_opts.ServeNewHealthServer(logger, server, mtmConfig.Health.SeverDialAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	return stopFn, nil
}

func NewMetamorphStore(dbConfig *config.DbConfig, tracingConfig *config.TracingConfig) (s store.MetamorphStore, err error) {
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

		opts := make([]func(postgreSQL *postgresql.PostgreSQL), 0)
		if tracingConfig != nil && tracingConfig.IsEnabled() {
			opts = append(opts, postgresql.WithTracing(tracingConfig.KeyValueAttributes))
		}

		s, err = postgresql.New(dbInfo, hostname, postgres.MaxIdleConns, postgres.MaxOpenConns, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbConfig.Mode)
	}

	return s, err
}

func initPeerManager(logger *slog.Logger, s store.MetamorphStore, arcConfig *config.ArcConfig) (p2p.PeerManagerI, *metamorph.PeerHandler, chan *metamorph.TxStatusMessage, error) {
	network, err := config.GetNetwork(arcConfig.Network)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get network: %v", err)
	}

	logger.Info("Assuming bitcoin network", "network", network)

	messageCh := make(chan *metamorph.TxStatusMessage, 10000)
	var pmOpts []p2p.PeerManagerOptions
	if arcConfig.Metamorph.MonitorPeers {
		pmOpts = append(pmOpts, p2p.WithRestartUnhealthyPeers())
	}

	pm := p2p.NewPeerManager(logger.With(slog.String("module", "peer-handler")), network, pmOpts...)

	peerHandler := metamorph.NewPeerHandler(s, messageCh)

	peerOpts := []p2p.PeerOptions{
		p2p.WithRetryReadWriteMessageInterval(5 * time.Second),
		p2p.WithPingInterval(30*time.Second, 1*time.Minute),
	}
	if version.Version != "" {
		peerOpts = append(peerOpts, p2p.WithUserAgent("ARC", version.Version))
	}

	for _, peerSetting := range arcConfig.Broadcasting.Unicast.Peers {
		peerURL, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error getting peer url: %v", err)
		}

		var peer *p2p.Peer
		peer, err = p2p.NewPeer(logger.With(slog.String("module", "peer")), peerURL, peerHandler, network, peerOpts...)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error creating peer %s: %v", peerURL, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			return nil, nil, nil, fmt.Errorf("error adding peer %s: %v", peerURL, err)
		}
	}

	return pm, peerHandler, messageCh, nil
}

func initGrpcCallbackerConn(address, prometheusEndpoint string, grpcMsgSize int, tracingConfig *config.TracingConfig) (callbacker_api.CallbackerAPIClient, error) {
	dialOpts, err := grpc_opts.GetGRPCClientOpts(prometheusEndpoint, grpcMsgSize, tracingConfig)
	if err != nil {
		return nil, err
	}
	callbackerConn, err := grpc.NewClient(address, dialOpts...)
	if err != nil {
		return nil, err
	}

	return callbacker_api.NewCallbackerAPIClient(callbackerConn), nil
}

func disposeMtm(l *slog.Logger, server *metamorph.Server, processor *metamorph.Processor,
	peerHandler *metamorph.PeerHandler, mqClient metamorph.MessageQueue,
	metamorphStore store.MetamorphStore, healthServer *grpc_opts.GrpcServer,
	shutdownFns []func(),
) {
	// dispose the dependencies in the correct order:
	// 1. server - ensure no new request will be received
	// 2. processor - ensure all started job are complete
	// 3. peerHandler
	// 4. mqClient
	// 5. store
	// 6. healthServer
	// 7. run shutdown functions

	if server != nil {
		server.GracefulStop()
	}
	if processor != nil {
		processor.Shutdown()
	}
	if peerHandler != nil {
		peerHandler.Shutdown()
	}
	if mqClient != nil {
		mqClient.Shutdown()
	}

	if metamorphStore != nil {
		err := metamorphStore.Close(context.Background())
		if err != nil {
			l.Error("Could not close store", slog.String("err", err.Error()))
		}
	}

	if healthServer != nil {
		healthServer.GracefulStop()
	}

	for _, shutdownFn := range shutdownFns {
		shutdownFn()
	}
}
