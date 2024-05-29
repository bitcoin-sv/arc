package cmd

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/async"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	cfg "github.com/bitcoin-sv/arc/internal/config"
	"github.com/bitcoin-sv/arc/internal/nats_mq"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/libsv/go-p2p"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	maximumBlockSize = 4294967296 // 4Gb
	service          = "blocktx"
)

func StartBlockTx(logger *slog.Logger, tracingEnabled bool) (func(), error) {
	dbMode, err := cfg.GetString("blocktx.db.mode")
	if err != nil {
		return nil, err
	}

	// dbMode can be sqlite, sqlite_memory or postgres
	blockStore, err := NewBlocktxStore(dbMode, logger, tracingEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocktx store: %v", err)
	}

	recordRetentionDays, err := cfg.GetInt("blocktx.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	network, err := cfg.GetNetwork()
	if err != nil {
		return nil, err
	}

	natsURL, err := cfg.GetString("queueURL")
	if err != nil {
		return nil, err
	}

	registerTxInterval, err := cfg.GetDuration("blocktx.registerTxsInterval")
	if err != nil {
		return nil, err
	}

	fillGapsInterval, err := cfg.GetDuration("blocktx.fillGapsInterval")
	if err != nil {
		return nil, err
	}

	// The tx channel needs the capacity so that it could potentially buffer up to a certain nr of transactions per second
	const targetTps = 6000
	capacityRequired := int(registerTxInterval.Seconds() * targetTps)
	if capacityRequired < 100 {
		capacityRequired = 100
	}

	txChannel := make(chan []byte, capacityRequired)
	requestTxChannel := make(chan []byte, capacityRequired)

	natsClient, err := nats_mq.NewNatsClient(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", natsURL, err)
	}

	maxBatchSize, err := cfg.GetInt("blocktx.mq.txsMinedMaxBatchSize")
	if err != nil {
		return nil, err
	}

	mqOpts := []func(handler *async.MQClient){
		async.WithMaxBatchSize(maxBatchSize),
	}

	if tracingEnabled {
		mqOpts = append(mqOpts, async.WithTracer())
	}

	mqClient := async.NewNatsMQClient(natsClient, txChannel, requestTxChannel, mqOpts...)

	err = mqClient.SubscribeRegisterTxs()
	if err != nil {
		return nil, err
	}

	err = mqClient.SubscribeRequestTxs()
	if err != nil {
		return nil, err
	}

	peerHandlerOpts := []func(handler *blocktx.PeerHandler){
		blocktx.WithRetentionDays(recordRetentionDays),
		blocktx.WithTxChan(txChannel),
		blocktx.WithRequestTxChan(requestTxChannel),
		blocktx.WithRegisterTxsInterval(registerTxInterval),
		blocktx.WithMessageQueueClient(mqClient),
		blocktx.WithFillGapsInterval(fillGapsInterval),
	}
	if tracingEnabled {
		peerHandlerOpts = append(peerHandlerOpts, blocktx.WithTracer())
	}

	peerHandler, err := blocktx.NewPeerHandler(logger, blockStore,
		peerHandlerOpts...)
	if err != nil {
		return nil, err
	}

	peerHandler.Start()

	pm := p2p.NewPeerManager(logger, network, p2p.WithExcessiveBlockSize(maximumBlockSize))

	peerSettings, err := cfg.GetPeerSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get peer settings: %v", err)
	}

	peerURLs := make([]string, len(peerSettings))
	for i, peerSetting := range peerSettings {
		peerUrl, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, fmt.Errorf("error getting peer url: %v", err)
		}
		peerURLs[i] = peerUrl
	}
	peers := make([]p2p.PeerI, len(peerURLs))

	opts := make([]p2p.PeerOptions, 0)

	opts = append(opts, p2p.WithMaximumMessageSize(maximumBlockSize))
	if version.Version != "" {
		opts = append(opts, p2p.WithUserAgent("ARC", version.Version))
	}

	opts = append(opts, p2p.WithRetryReadWriteMessageInterval(5*time.Second))

	for i, peerURL := range peerURLs {
		peer, err := p2p.NewPeer(logger, peerURL, peerHandler, network, opts...)
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

	server := blocktx.NewServer(blockStore, logger, peers)

	blocktxGRPCListenAddress, err := cfg.GetString("blocktx.listenAddr")
	if err != nil {
		return nil, err
	}

	grpcMessageSize, err := cfg.GetInt("grpcMessageSize")
	if err != nil {
		return nil, err
	}

	prometheusEndpoint := viper.GetString("prometheusEndpoint")

	err = server.StartGRPCServer(blocktxGRPCListenAddress, grpcMessageSize, prometheusEndpoint, logger)
	if err != nil {
		return nil, fmt.Errorf("GRPCServer failed: %v", err)
	}

	healthServer, err := StartHealthServerBlocktx(server, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	return func() {
		logger.Info("Shutting down blocktx store")

		err = mqClient.Shutdown()
		if err != nil {
			logger.Error("Failed to shutdown mqClient", slog.String("err", err.Error()))
		}

		err = blockStore.Close()
		if err != nil {
			logger.Error("Failed to close blocktx store", slog.String("err", err.Error()))
		}

		peerHandler.Shutdown()

		server.Shutdown()

		healthServer.Stop()
	}, nil
}

func StartHealthServerBlocktx(serv *blocktx.Server, logger *slog.Logger) (*grpc.Server, error) {
	gs := grpc.NewServer()

	grpc_health_v1.RegisterHealthServer(gs, serv) // registration
	// register your own services
	reflection.Register(gs)

	address, err := cfg.GetString("blocktx.healthServerDialAddr") //"localhost:8005"
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	go func() {

		logger.Info("GRPC health server listening", slog.String("address", address))
		err = gs.Serve(listener)
		if err != nil {
			logger.Error("GRPC health server failed to serve", slog.String("err", err.Error()))
		}
	}()

	return gs, nil
}

func NewBlocktxStore(dbMode string, logger *slog.Logger, tracingEnabled bool) (s store.BlocktxStore, err error) {
	switch dbMode {
	case DbModePostgres:
		dbHost, err := cfg.GetString("blocktx.db.postgres.host")
		if err != nil {
			return nil, err
		}
		dbPort, err := cfg.GetInt("blocktx.db.postgres.port")
		if err != nil {
			return nil, err
		}
		dbName, err := cfg.GetString("blocktx.db.postgres.name")
		if err != nil {
			return nil, err
		}
		dbUser, err := cfg.GetString("blocktx.db.postgres.user")
		if err != nil {
			return nil, err
		}
		dbPassword, err := cfg.GetString("blocktx.db.postgres.password")
		if err != nil {
			return nil, err
		}
		sslMode, err := cfg.GetString("blocktx.db.postgres.sslMode")
		if err != nil {
			return nil, err
		}
		idleConns, err := cfg.GetInt("blocktx.db.postgres.maxIdleConns")
		if err != nil {
			return nil, err
		}
		maxOpenConns, err := cfg.GetInt("blocktx.db.postgres.maxOpenConns")
		if err != nil {
			return nil, err
		}

		logger.Info(fmt.Sprintf("db connection: user=%s dbname=%s host=%s port=%d sslmode=%s", dbUser, dbName, dbHost, dbPort, sslMode))

		dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=%s", dbUser, dbPassword, dbName, dbHost, dbPort, sslMode)

		var postgresOpts []func(handler *postgresql.PostgreSQL)
		if tracingEnabled {
			postgresOpts = append(postgresOpts, postgresql.WithTracer())
		}

		s, err = postgresql.New(dbInfo, idleConns, maxOpenConns, postgresOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbMode)
	}

	return s, err
}
