package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/nats_connection"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/libsv/go-p2p"
)

const (
	maximumBlockSize      = 4294967296 // 4Gb
	blockProcessingBuffer = 100
)

func StartBlockTx(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	logger = logger.With(slog.String("service", "blocktx"))
	logger.Info("Starting")

	tracingEnabled := arcConfig.Tracing != nil
	btxConfig := arcConfig.Blocktx

	var (
		blockStore   store.BlocktxStore
		mqClient     blocktx.MessageQueueClient
		processor    *blocktx.Processor
		pm           p2p.PeerManagerI
		server       *blocktx.Server
		healthServer *grpc_opts.GrpcServer
		workers      *blocktx.BackgroundWorkers

		err error
	)

	stopFn := func() {
		logger.Info("Shutting down blocktx")
		disposeBlockTx(logger, server, processor, pm, mqClient, blockStore, healthServer, workers)
		logger.Info("Shutted down")
	}

	network, err := config.GetNetwork(arcConfig.Network)
	if err != nil {
		return nil, err
	}

	blockStore, err = NewBlocktxStore(logger, btxConfig.Db, tracingEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocktx store: %v", err)
	}

	registerTxsChan := make(chan []byte, chanBufferSize)
	requestTxChannel := make(chan []byte, chanBufferSize)

	natsConnection, err := nats_connection.New(arcConfig.MessageQueue.URL, logger)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
	}

	if arcConfig.MessageQueue.Streaming.Enabled {
		opts := []nats_jetstream.Option{nats_jetstream.WithSubscribedTopics(blocktx.RegisterTxTopic, blocktx.RequestTxTopic)}
		if arcConfig.MessageQueue.Streaming.FileStorage {
			opts = append(opts, nats_jetstream.WithFileStorage())
		}

		mqClient, err = nats_jetstream.New(natsConnection, logger,
			[]string{blocktx.MinedTxsTopic, blocktx.RegisterTxTopic, blocktx.RequestTxTopic},
			opts...,
		)
		if err != nil {
			stopFn()
			return nil, fmt.Errorf("failed to create nats client: %v", err)
		}
	} else {
		mqClient = nats_core.New(natsConnection, nats_core.WithLogger(logger))
	}

	processorOpts := []func(handler *blocktx.Processor){
		blocktx.WithRetentionDays(btxConfig.RecordRetentionDays),
		blocktx.WithRegisterTxsChan(registerTxsChan),
		blocktx.WithRequestTxChan(requestTxChannel),
		blocktx.WithRegisterTxsInterval(btxConfig.RegisterTxsInterval),
		blocktx.WithMessageQueueClient(mqClient),
	}
	if tracingEnabled {
		processorOpts = append(processorOpts, blocktx.WithTracer())
	}

	blockRequestCh := make(chan blocktx.BlockRequest, blockProcessingBuffer)
	blockProcessCh := make(chan *p2p.BlockMessage, blockProcessingBuffer)

	processor, err = blocktx.NewProcessor(logger, blockStore, blockRequestCh, blockProcessCh, processorOpts...)
	if err != nil {
		stopFn()
		return nil, err
	}

	err = processor.Start()
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start peer handler: %v", err)
	}

	peerOpts := []p2p.PeerOptions{
		p2p.WithMaximumMessageSize(maximumBlockSize),
		p2p.WithRetryReadWriteMessageInterval(5 * time.Second),
		p2p.WithPingInterval(30*time.Second, 1*time.Minute),
	}

	if version.Version != "" {
		peerOpts = append(peerOpts, p2p.WithUserAgent("ARC", version.Version))
	}

	pmOpts := []p2p.PeerManagerOptions{p2p.WithExcessiveBlockSize(maximumBlockSize)}
	if arcConfig.Metamorph.MonitorPeers {
		pmOpts = append(pmOpts, p2p.WithRestartUnhealthyPeers())
	}

	pm = p2p.NewPeerManager(logger.With(slog.String("module", "peer-mng")), network, pmOpts...)
	peers := make([]p2p.PeerI, len(arcConfig.Peers))

	peerHandler := blocktx.NewPeerHandler(logger, blockRequestCh, blockProcessCh)

	for i, peerSetting := range arcConfig.Peers {
		peerURL, err := peerSetting.GetP2PUrl()
		if err != nil {
			stopFn()
			return nil, fmt.Errorf("error getting peer url: %v", err)
		}

		peer, err := p2p.NewPeer(logger.With(slog.String("module", "peer")), peerURL, peerHandler, network, peerOpts...)
		if err != nil {
			stopFn()
			return nil, fmt.Errorf("error creating peer %s: %v", peerURL, err)
		}
		err = pm.AddPeer(peer)
		if err != nil {
			stopFn()
			return nil, fmt.Errorf("error adding peer: %v", err)
		}

		peers[i] = peer
	}

	workers = blocktx.NewBackgroundWorkers(blockStore, logger)
	workers.StartFillGaps(peers, btxConfig.FillGapsInterval, btxConfig.RecordRetentionDays, blockRequestCh)

	server, err = blocktx.NewServer(arcConfig.PrometheusEndpoint, arcConfig.GrpcMessageSize, logger,
		blockStore, pm, btxConfig.MaxAllowedBlockHeightMismatch)
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

func NewBlocktxStore(logger *slog.Logger, dbConfig *config.DbConfig, tracingEnabled bool) (s store.BlocktxStore, err error) {
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
		if tracingEnabled {
			postgresOpts = append(postgresOpts, postgresql.WithTracer())
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

func disposeBlockTx(l *slog.Logger, server *blocktx.Server, processor *blocktx.Processor,
	pm p2p.PeerManagerI, mqClient blocktx.MessageQueueClient,
	store store.BlocktxStore, healthServer *grpc_opts.GrpcServer, workers *blocktx.BackgroundWorkers) {

	// dispose the dependencies in the correct order:
	// 1. server - ensure no new requests will be received
	// 2. background workers
	// 3. processor - ensure all started job are complete
	// 4. peer manager
	// 5. mqClient
	// 6. store
	// 7. healthServer

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
}
