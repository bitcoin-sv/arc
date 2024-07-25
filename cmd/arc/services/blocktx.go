package cmd

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/async"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/nats_mq"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/libsv/go-p2p"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	maximumBlockSize = 4294967296 // 4Gb
)

func StartBlockTx(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	tracingEnabled := arcConfig.Tracing != nil
	btxConfig := arcConfig.Blocktx

	network, err := config.GetNetwork(arcConfig.Network)
	if err != nil {
		return nil, err
	}

	blockStore, err := NewBlocktxStore(logger, btxConfig.Db, tracingEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocktx store: %v", err)
	}

	// The tx channel needs the capacity so that it could potentially buffer up to a certain nr of transactions per second
	const targetTps = 6000
	capacityRequired := int(btxConfig.RegisterTxsInterval.Seconds() * targetTps)
	if capacityRequired < 100 {
		capacityRequired = 100
	}

	registerTxsChan := make(chan []byte, capacityRequired)
	requestTxChannel := make(chan []byte, capacityRequired)

	natsClient, err := nats_mq.NewNatsClient(arcConfig.QueueURL, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.QueueURL, err)
	}

	mqOpts := []func(handler *async.MQClient){
		async.WithMaxBatchSize(btxConfig.MessageQueue.TxsMinedMaxBatchSize),
		async.WithRequestTxsChan(requestTxChannel),
		async.WithRegisterTxsChan(registerTxsChan),
	}

	if tracingEnabled {
		mqOpts = append(mqOpts, async.WithTracer())
	}

	mqClient := async.NewNatsMQClient(natsClient, mqOpts...)

	err = mqClient.SubscribeRegisterTxs()
	if err != nil {
		return nil, err
	}

	err = mqClient.SubscribeRequestTxs()
	if err != nil {
		return nil, err
	}

	peerHandlerOpts := []func(handler *blocktx.PeerHandler){
		blocktx.WithRetentionDays(btxConfig.RecordRetentionDays),
		blocktx.WithRegisterTxsChan(registerTxsChan),
		blocktx.WithRequestTxChan(requestTxChannel),
		blocktx.WithRegisterTxsInterval(btxConfig.RegisterTxsInterval),
		blocktx.WithMessageQueueClient(mqClient),
		blocktx.WithFillGapsInterval(btxConfig.FillGapsInterval),
	}
	if tracingEnabled {
		peerHandlerOpts = append(peerHandlerOpts, blocktx.WithTracer())
	}

	peerHandler, err := blocktx.NewPeerHandler(logger, blockStore, peerHandlerOpts...)
	if err != nil {
		return nil, err
	}

	peerHandler.Start()

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

	pm := p2p.NewPeerManager(logger, network, pmOpts...)
	peers := make([]p2p.PeerI, len(arcConfig.Peers))

	for i, peerSetting := range arcConfig.Peers {
		peerURL, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, fmt.Errorf("error getting peer url: %v", err)
		}
		peer, err := p2p.NewPeer(logger, peerURL, peerHandler, network, peerOpts...)
		if err != nil {
			return nil, fmt.Errorf("error creating peer %s: %v", peerURL, err)
		}
		err = pm.AddPeer(peer)
		if err != nil {
			return nil, fmt.Errorf("error adding peer: %v", err)
		}

		peers[i] = peer
	}

	peerHandler.StartFillGaps(peers)

	server := blocktx.NewServer(blockStore, logger, pm, btxConfig.MaxAllowedBlockHeightMismatch)

	err = server.StartGRPCServer(btxConfig.ListenAddr, arcConfig.GrpcMessageSize, arcConfig.PrometheusEndpoint, logger)
	if err != nil {
		return nil, fmt.Errorf("GRPCServer failed: %v", err)
	}

	healthServer, err := StartHealthServerBlocktx(server, logger, btxConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	return func() {
		logger.Info("Shutting down blocktx store")

		peerHandler.Shutdown()

		server.Shutdown()

		err = mqClient.Shutdown()
		if err != nil {
			logger.Error("Failed to shutdown mqClient", slog.String("err", err.Error()))
		}

		err = blockStore.Close()
		if err != nil {
			logger.Error("Failed to close blocktx store", slog.String("err", err.Error()))
		}

		healthServer.Stop()
	}, nil
}

func StartHealthServerBlocktx(serv *blocktx.Server, logger *slog.Logger, btxConfig *config.BlocktxConfig) (*grpc.Server, error) {
	gs := grpc.NewServer()

	grpc_health_v1.RegisterHealthServer(gs, serv) // registration
	// register your own services
	reflection.Register(gs)

	listener, err := net.Listen("tcp", btxConfig.HealthServerDialAddr)
	if err != nil {
		return nil, err
	}

	go func() {
		logger.Info("GRPC health server listening", slog.String("address", btxConfig.HealthServerDialAddr))
		err = gs.Serve(listener)
		if err != nil {
			logger.Error("GRPC health server failed to serve", slog.String("err", err.Error()))
		}
	}()

	return gs, nil
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
