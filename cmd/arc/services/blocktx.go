package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/mcast"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_core"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	maximumBlockSize      = 4294967296 // 4Gb
	blockProcessingBuffer = 100
)

func StartBlockTx(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	logger = logger.With(slog.String("service", "blocktx"))
	logger.Info("Starting")

	btxConfig := arcConfig.Blocktx

	var (
		blockStore    store.BlocktxStore
		mqClient      blocktx.MessageQueueClient
		processor     *blocktx.Processor
		pm            *p2p.PeerManager
		mcastListener *mcast.Listener
		server        *blocktx.Server
		healthServer  *grpc_opts.GrpcServer
		workers       *blocktx.BackgroundWorkers

		err error
	)

	shutdownFns := make([]func(), 0)
	processorOpts := make([]func(handler *blocktx.Processor), 0)

	if arcConfig.IsTracingEnabled() {
		cleanup, err := tracing.Enable(logger, "blocktx", arcConfig.Tracing.DialAddr, arcConfig.Tracing.Sample)
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

		processorOpts = append(processorOpts, blocktx.WithTracer(attributes...))
	}

	stopFn := func() {
		logger.Info("Shutting down blocktx")
		disposeBlockTx(logger, server, processor, pm, mcastListener, mqClient, blockStore, healthServer, workers, shutdownFns)
		logger.Info("Shutdown complete")
	}

	blockStore, err = NewBlocktxStore(logger, btxConfig.Db, arcConfig.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocktx store: %v", err)
	}

	registerTxsChan := make(chan []byte, chanBufferSize)

	natsConnection, err := nats_connection.New(arcConfig.MessageQueue.URL, logger)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
	}

	if arcConfig.MessageQueue.Streaming.Enabled {
		opts := []nats_jetstream.Option{
			nats_jetstream.WithSubscribedWorkQueuePolicy(blocktx.RegisterTxTopic),
			nats_jetstream.WithWorkQueuePolicy(blocktx.MinedTxsTopic),
		}
		if arcConfig.MessageQueue.Streaming.FileStorage {
			opts = append(opts, nats_jetstream.WithFileStorage())
		}

		if arcConfig.Tracing.Enabled {
			opts = append(opts, nats_jetstream.WithTracer(arcConfig.Tracing.KeyValueAttributes...))
		}

		mqClient, err = nats_jetstream.New(natsConnection, logger,
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
		mqClient = nats_core.New(natsConnection, opts...)
	}

	processorOpts = append(processorOpts,
		blocktx.WithRetentionDays(btxConfig.RecordRetentionDays),
		blocktx.WithRegisterTxsChan(registerTxsChan),
		blocktx.WithRegisterTxsInterval(btxConfig.RegisterTxsInterval),
		blocktx.WithMessageQueueClient(mqClient),
		blocktx.WithMaxBlockProcessingDuration(btxConfig.MaxBlockProcessingDuration),
		blocktx.WithIncomingIsLongest(btxConfig.IncomingIsLongest),
	)

	blockRequestCh := make(chan blocktx_p2p.BlockRequest, blockProcessingBuffer)
	blockProcessCh := make(chan *bcnet.BlockMessage, blockProcessingBuffer)

	processor, err = blocktx.NewProcessor(logger, blockStore, blockRequestCh, blockProcessCh, processorOpts...)
	if err != nil {
		stopFn()
		return nil, err
	}

	err = processor.Start(arcConfig.Prometheus.IsEnabled())
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start prometheus: %v", err)
	}

	pm, mcastListener, err = setupBcNetworkCommunication(logger, arcConfig, blockStore, blockRequestCh, blockProcessCh)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to establish connection with network: %v", err)
	}

	if btxConfig.FillGaps != nil && btxConfig.FillGaps.Enabled {
		workers = blocktx.NewBackgroundWorkers(blockStore, logger)
		workers.StartFillGaps(pm.GetPeers(), btxConfig.FillGaps.Interval, btxConfig.RecordRetentionDays, blockRequestCh)
	}

	server, err = blocktx.NewServer(arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, logger,
		blockStore, pm, btxConfig.MaxAllowedBlockHeightMismatch, arcConfig.Tracing)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("create GRPCServer failed: %v", err)
	}

	err = server.ListenAndServe(btxConfig.ListenAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("serve GRPCServer failed: %v", err)
	}

	healthServer, err = grpc_opts.ServeNewHealthServer(logger, server, btxConfig.HealthServerDialAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	return stopFn, nil
}

func NewBlocktxStore(logger *slog.Logger, dbConfig *config.DbConfig, tracingConfig *config.TracingConfig) (s store.BlocktxStore, err error) {
	switch dbConfig.Mode {
	case DbModePostgres:
		postgres := dbConfig.Postgres

		logger.Info(fmt.Sprintf(
			"db connection: user=%s dbname=%s host=%s port=%d sslmode=%s",
			postgres.User, postgres.Name, postgres.Host, postgres.Port, postgres.SslMode,
		))

		dbInfo := fmt.Sprintf(
			"user=%s password=%s dbname=%s host=%s port=%d sslmode=%s",
			postgres.User, postgres.Password, postgres.Name, postgres.Host, postgres.Port, postgres.SslMode,
		)

		var postgresOpts []func(handler *postgresql.PostgreSQL)
		if tracingConfig != nil && tracingConfig.IsEnabled() {
			postgresOpts = append(postgresOpts, postgresql.WithTracer(tracingConfig.KeyValueAttributes...))
		}

		s, err = postgresql.New(dbInfo, postgres.MaxIdleConns, postgres.MaxOpenConns, postgresOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbConfig.Mode)
	}

	return s, err
}

func setupBcNetworkCommunication(l *slog.Logger, arcConfig *config.ArcConfig, store store.BlocktxStore, blockRequestCh chan<- blocktx_p2p.BlockRequest, blockProcessCh chan<- *bcnet.BlockMessage) (manager *p2p.PeerManager, mcastListener *mcast.Listener, err error) {
	defer func() {
		// cleanup on error
		if err == nil {
			return
		}

		if manager != nil {
			manager.Shutdown()
		}

		if mcastListener != nil {
			mcastListener.Disconnect()
		}
	}()

	// p2p global setting
	p2p.SetExcessiveBlockSize(maximumBlockSize)

	cfg := arcConfig.Blocktx.BlockchainNetwork
	network, err := config.GetNetwork(cfg.Network)
	if err != nil {
		return
	}

	var msgHandler p2p.MessageHandlerI

	if cfg.Mode == "classic" {
		msgHandler = blocktx_p2p.NewMsgHandler(l, blockRequestCh, blockProcessCh)
	} else if cfg.Mode == "hybrid" {
		l.Info("!!! Blocktx will communicate with blockchain in HYBRID mode (via p2p and multicast groups) !!!")
		msgHandler = blocktx_p2p.NewHybridMsgHandler(l, blockProcessCh)
	} else {
		return nil, nil, fmt.Errorf("unsupported communication type: %s", cfg.Mode)
	}

	// connect to peers
	var managerOpts []p2p.PeerManagerOptions
	if arcConfig.Blocktx.MonitorPeers {
		managerOpts = append(managerOpts, p2p.WithRestartUnhealthyPeers())
	}

	manager = p2p.NewPeerManager(l.With(slog.String("module", "peer-mng")), network, managerOpts...)
	peers, err := connectToPeers(l, network, msgHandler, cfg.Peers, p2p.WithMaximumMessageSize(maximumBlockSize))
	if err != nil {
		return
	}

	for _, p := range peers {
		if err = manager.AddPeer(p); err != nil {
			return
		}
	}

	// connect to mcast
	if cfg.Mode == "hybrid" {
		if cfg.Mcast == nil {
			return manager, mcastListener, errors.New("mcast config is required")
		}

		// TODO: add net interfaces
		mcastListener = mcast.NewMcastListener(l, cfg.Mcast.McastBlock.Address, network, store, blockProcessCh)
		ok := mcastListener.Connect()
		if !ok {
			return manager, nil, fmt.Errorf("error connecting to mcast %s: %w", cfg.Mcast.McastBlock, err)
		}
	}

	return
}

func connectToPeers(l *slog.Logger, network wire.BitcoinNet, msgHandler p2p.MessageHandlerI, peersConfig []*config.PeerConfig, additionalOpts ...p2p.PeerOptions) (peers []*p2p.Peer, err error) {
	defer func() {
		// cleanup on error
		if err == nil {
			return
		}

		for _, p := range peers {
			p.Shutdown()
		}
	}()

	opts := []p2p.PeerOptions{
		p2p.WithPingInterval(30*time.Second, 2*time.Minute),
	}

	if version.Version != "" {
		opts = append(opts, p2p.WithUserAgent("ARC", version.Version))
	}

	opts = append(opts, additionalOpts...)

	for _, settings := range peersConfig {
		url, err := settings.GetP2PUrl()
		if err != nil {
			return nil, fmt.Errorf("error getting peer url: %w", err)
		}

		p := p2p.NewPeer(
			l.With(slog.String("module", "peer")),
			msgHandler,
			url,
			network,
			opts...)
		ok := p.Connect()
		if !ok {
			return nil, fmt.Errorf("error connecting peer %s: %w", url, err)
		}

		peers = append(peers, p)
	}

	return
}

func disposeBlockTx(l *slog.Logger, server *blocktx.Server, processor *blocktx.Processor,
	pm *p2p.PeerManager, mcastListener *mcast.Listener, mqClient blocktx.MessageQueueClient,
	store store.BlocktxStore, healthServer *grpc_opts.GrpcServer, workers *blocktx.BackgroundWorkers,
	shutdownFns []func(),
) {
	// dispose the dependencies in the correct order:
	// 1. server - ensure no new requests will be received
	// 2. background workers
	// 3. processor - ensure all started job are complete
	// 4. peer manager
	// 5. mqClient
	// 6. store
	// 7. healthServer
	// 8. run shutdown functions

	if server != nil {
		server.GracefulStop()
	}
	if workers != nil {
		workers.GracefulStop()
	}
	if processor != nil {
		processor.Shutdown()
	}
	if pm != nil {
		pm.Shutdown()
	}
	if mcastListener != nil {
		mcastListener.Disconnect()
	}
	if mqClient != nil {
		mqClient.Shutdown()
	}

	if store != nil {
		err := store.Close()
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
