package services

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/pkg/message_queue"
	"github.com/libsv/go-p2p/wire"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/mcast"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	maximumBlockSize      = 4294967296 // 4Gb
	blockProcessingBuffer = 100
	p2pConnectionTimeout  = 30 * time.Second
	minConnections        = 1
)

func StartBlockTx(logger *slog.Logger, btxCfg *config.BlocktxConfig, commonCfg *config.CommonConfig) (func(), error) {
	logger = logger.With(slog.String("service", "blocktx"))
	logger.Info("Starting")

	shutdownFns := make([]func(), 0)
	processorOpts := make([]func(handler *blocktx.Processor), 0)

	if commonCfg.IsTracingEnabled() {
		cleanup, err := tracing.Enable(logger, "blocktx", commonCfg.Tracing.DialAddr, commonCfg.Tracing.Sample)
		if err != nil {
			logger.Error("failed to enable tracing", slog.String("err", err.Error()))
		} else {
			shutdownFns = append(shutdownFns, cleanup)
		}

		attributes := commonCfg.Tracing.KeyValueAttributes
		hostname, err := os.Hostname()
		if err == nil {
			hostnameAttr := attribute.String("hostname", hostname)
			attributes = append(attributes, hostnameAttr)
		}

		processorOpts = append(processorOpts, blocktx.WithTracer(attributes...))
	}

	var stoppable global.Stoppables
	var stoppableWithError global.StoppablesWithError

	stopFn := func() {
		logger.Info("Shutting down blocktx")
		for _, shutdownFn := range shutdownFns {
			shutdownFn()
		}
		stoppable.Shutdown()
		stoppableWithError.Shutdown(logger)
		logger.Info("Shutdown metamorph complete")
		logger.Info("Shutdown blocktx complete")
	}

	blockStore, err := newBlocktxStore(logger, btxCfg.Db, commonCfg.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocktx store: %v", err)
	}
	stoppableWithError = append(stoppableWithError, blockStore)

	registerTxsChan := make(chan []byte, chanBufferSize)

	mqClient, err := mq.NewMqClient(logger, commonCfg.MessageQueue)
	if err != nil {
		return nil, err
	}
	stoppable = append(stoppable, mqClient)

	subscribeAdapter := blocktx.NewMessageSubscribeAdapter(mqClient, logger)
	err = subscribeAdapter.Start(registerTxsChan)
	if err != nil {
		stopFn()
		return nil, err
	}
	stoppable = append(stoppable, subscribeAdapter)

	minedTxsChan := make(chan *blocktx_api.TransactionBlocks, chanBufferSize)
	publishAdapter := message_queue.NewPublishAdapter(mqClient, logger)
	publishAdapter.StartPublishMarshal(mq.MinedTxsTopic)
	go func() {
		for msg := range minedTxsChan {
			publishAdapter.Publish(msg)
		}
	}()
	stoppable = append(stoppable, publishAdapter)

	processorOpts = append(processorOpts,
		blocktx.WithRetentionDays(btxCfg.RecordRetentionDays),
		blocktx.WithRegisterTxsChan(registerTxsChan),
		blocktx.WithRegisterTxsInterval(btxCfg.RegisterTxsInterval),
		blocktx.WithMinedTxsChan(minedTxsChan),
		blocktx.WithMaxBlockProcessingDuration(btxCfg.MaxBlockProcessingDuration),
		blocktx.WithIncomingIsLongest(btxCfg.IncomingIsLongest),
	)

	blockRequestCh := make(chan blocktx_p2p.BlockRequest, blockProcessingBuffer)
	blockProcessCh := make(chan *bcnet.BlockMessagePeer, blockProcessingBuffer)

	processor, err := blocktx.NewProcessor(logger, blockStore, blockRequestCh, blockProcessCh, processorOpts...)
	if err != nil {
		stopFn()
		return nil, err
	}
	stoppable = append(stoppable, processor)

	err = processor.Start()
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start prometheus: %v", err)
	}

	pm, mcastListener, err := setupBcNetworkCommunication(logger, btxCfg, blockStore, blockRequestCh, minConnections, blockProcessCh)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to establish connection with network: %v", err)
	}
	stoppable = append(stoppable, pm)
	stoppable = append(stoppable, mcastListener)

	if commonCfg.Prometheus.IsEnabled() {
		statsCollector := blocktx.NewStatsCollector(logger, pm, blockStore, btxCfg.RecordRetentionDays)
		stoppable = append(stoppable, statsCollector)
		err = statsCollector.Start()
		if err != nil {
			stopFn()
			return nil, fmt.Errorf("failed to start stats collector: %v", err)
		}
	}

	if btxCfg.FillGaps != nil && btxCfg.FillGaps.Enabled {
		workers := blocktx.NewBackgroundWorkers(blockStore, logger)
		stoppable = append(stoppable, workers)
		workers.StartFillGaps(pm.GetPeers(), btxCfg.FillGaps.Interval, btxCfg.RecordRetentionDays, blockRequestCh)
	}

	if btxCfg.UnorphanRecentWrongOrphans != nil && btxCfg.UnorphanRecentWrongOrphans.Enabled {
		workers := blocktx.NewBackgroundWorkers(blockStore, logger)
		stoppable = append(stoppable, workers)
		workers.StartUnorphanRecentWrongOrphans(btxCfg.UnorphanRecentWrongOrphans.Interval)
	}

	serverCfg := grpc_utils.ServerConfig{
		PrometheusEndpoint: commonCfg.Prometheus.Endpoint,
		MaxMsgSize:         commonCfg.GrpcMessageSize,
		TracingConfig:      commonCfg.Tracing,
		Name:               "blocktx",
	}

	server, err := blocktx.NewServer(logger, blockStore, pm, processor, serverCfg, btxCfg.MaxAllowedBlockHeightMismatch, mqClient, btxCfg.RecordRetentionDays)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("create GRPCServer failed: %v", err)
	}
	stoppable = append(stoppable, server)

	err = server.ListenAndServe(btxCfg.ListenAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("serve GRPCServer failed: %v", err)
	}

	return stopFn, nil
}

func newBlocktxStore(logger *slog.Logger, dbConfig *config.DbConfig, tracingConfig *config.TracingConfig) (s *postgresql.PostgreSQL, err error) {
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

// setupBcNetworkCommunication initializes the Bloctx blockchain network communication layer, configuring it
// to operate in either classic (P2P-only) or hybrid (P2P and multicast) mode.
//
// Parameters:
// - `l *slog.Logger`: Logger instance for logging events.
// - `bloctxCfg *config.BlocktxConfig`: Configuration object containing blockchain network settings.
// - `store store.BlocktxStore`: A storage interface for blockchain transactions.
// - `blockRequestCh chan<- blocktx_p2p.BlockRequest`: Channel for handling block requests.
// - `blockProcessCh chan<- *bcnet.BlockMessage`: Channel for processing block messages.
//
// Returns:
// - `manager *p2p.PeerManager`: Manages P2P peers.
// - `mcastListener *mcast.Listener`: Handles multicast communication, or `nil` if not in hybrid mode.
// - `err error`: Error if any issue occurs during setup.
//
// Key Details:
// - **Mode Handling**:
//   - `"classic"` mode uses `blocktx_p2p.NewMsgHandler` for P2P communication only.
//   - `"hybrid"` mode uses `blocktx_p2p.NewHybridMsgHandler` for P2P and multicast communication.
//
// - **Error Cleanup**: Ensures resources like peers and multicast listeners are properly cleaned up on errors.
// - **Peer Management**: Connects to configured peers and initializes a PeerManager for handling P2P connections.
// - **Multicast Communication**: In hybrid mode, joins a multicast group for blockchain updates and uses correct P2P message handler.
//
// Message Handlers:
// - `blocktx_p2p.NewMsgHandler`: Used in classic mode, handles all blockchain communication exclusively via P2P.
// - `blocktx_p2p.NewHybridMsgHandler`: Used in hybrid mode, seamlessly integrates P2P communication with multicast group updates.
func setupBcNetworkCommunication(l *slog.Logger, bloctxCfg *config.BlocktxConfig, store store.BlocktxStore, blockRequestCh chan<- blocktx_p2p.BlockRequest, minConnections int, blockProcessCh chan<- *bcnet.BlockMessagePeer) (manager *p2p.PeerManager, mcastListener *mcast.Listener, err error) {
	// p2p global setting
	p2p.SetExcessiveBlockSize(maximumBlockSize)

	cfg := bloctxCfg.BlockchainNetwork
	network, err := config.GetNetwork(cfg.Network)
	if err != nil {
		return
	}

	var msgHandler p2p.MessageHandlerI

	switch cfg.Mode {
	case "classic":
		msgHandler = blocktx_p2p.NewMsgHandler(l, blockRequestCh, blockProcessCh)
	case "hybrid":
		l.Info("!!! Blocktx will communicate with blockchain in HYBRID mode (via p2p and multicast groups) !!!")
		msgHandler = blocktx_p2p.NewHybridMsgHandler(l, blockProcessCh)
	default:
		return nil, nil, fmt.Errorf("unsupported communication type: %s", cfg.Mode)
	}

	// connect to peers
	var managerOpts []p2p.PeerManagerOptions
	if bloctxCfg.MonitorPeers {
		managerOpts = append(managerOpts, p2p.WithRestartUnhealthyPeers())
	}

	manager = p2p.NewPeerManager(l.With(slog.String("module", "peer-mng")), network, managerOpts...)

	connectionsReady := make(chan struct{})
	go connectToPeers(l, manager, connectionsReady, minConnections, network, msgHandler, cfg.Peers, p2p.WithMaximumMessageSize(maximumBlockSize))

	// wait until min peer connections are ready and then continue startup while remaining peers connect
	<-connectionsReady
	l.Info("current open peer connections", slog.Uint64("count", uint64(manager.CountConnectedPeers())))

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

func connectToPeers(l *slog.Logger, manager *p2p.PeerManager, connectionsReady chan struct{}, minConnections int, network wire.BitcoinNet, msgHandler p2p.MessageHandlerI, peersConfig []*config.PeerConfig, additionalOpts ...p2p.PeerOptions) {
	var peers []p2p.PeerI
	var err error
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
		opts = append(opts,
			p2p.WithUserAgent("ARC", version.Version),
			p2p.WithConnectionTimeout(p2pConnectionTimeout),
		)
	}

	opts = append(opts, additionalOpts...)
	var connectedPeers int

	for _, settings := range peersConfig {
		url, err := settings.GetP2PUrl()
		if err != nil {
			l.Error("error getting peer url: ", slog.String("err", err.Error()))
		}

		p := p2p.NewPeer(
			l.With(slog.String("module", "peer")),
			msgHandler,
			url,
			network,
			opts...)

		err = manager.AddPeer(p)
		if err != nil {
			l.Error("could not add a new peer to the manager ", slog.String("err", err.Error()))
			continue
		}

		// collect peers just for shutdown
		peers = append(peers, p)

		if !p.Connect() {
			l.Warn("Failed to connect to peer", slog.String("url", url))
			continue
		}

		connectedPeers++
		// notify if we have enough connections and can continue startup
		if connectedPeers == minConnections {
			close(connectionsReady)
		}
	}
}
