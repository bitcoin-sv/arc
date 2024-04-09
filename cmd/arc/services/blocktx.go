package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/async/nats_mq"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	cfg "github.com/bitcoin-sv/arc/internal/helpers"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/libsv/go-p2p"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	maximumBlockSize = 4294967296 // 4Gb
	service          = "blocktx"
)

func StartBlockTx(logger *slog.Logger) (func(), error) {
	dbMode, err := cfg.GetString("blocktx.db.mode")
	if err != nil {
		return nil, err
	}

	// dbMode can be sqlite, sqlite_memory or postgres
	blockStore, err := NewBlocktxStore(dbMode, logger)
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

	mqClient := nats_mq.NewNatsMQClient(natsClient, txChannel, requestTxChannel, nats_mq.WithMaxBatchSize(maxBatchSize))
	if err != nil {
		return nil, fmt.Errorf("failed to create message queue client: %v", err)
	}

	err = mqClient.SubscribeRegisterTxs()
	if err != nil {
		return nil, err
	}

	// err = mqClient.SubscribeRequestTxs()
	// if err != nil {
	// 	return nil, err
	// }

	peerHandlerOpts := []func(handler *blocktx.PeerHandler){
		blocktx.WithRetentionDays(recordRetentionDays),
		blocktx.WithTxChan(txChannel),
		blocktx.WithRequestTxChan(txChannel),
		blocktx.WithRegisterTxsInterval(registerTxInterval),
		blocktx.WithMessageQueueClient(mqClient),
	}

	var tp *trace.TracerProvider

	if viper.GetBool("blocktx.tracing.enabled") {
		ctx := context.Background()

		tracingAddr, err := cfg.GetString("blocktx.tracing.dialAddr")
		if err != nil {
			return nil, err
		}

		exporter, err := tracing.NewExporter(ctx, tracingAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize exporter: %v", err)
		}

		tp, err = tracing.NewTraceProvider(exporter, service)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace provider: %v", err)
		}

		otel.SetTracerProvider(tp)

		peerHandlerOpts = append(peerHandlerOpts, blocktx.WithTracer(tp.Tracer(service)))
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

	blockTxServer := blocktx.NewServer(blockStore, logger, peers)

	address, err := cfg.GetString("blocktx.listenAddr")
	if err != nil {
		return nil, err
	}
	go func() {
		if err = blockTxServer.StartGRPCServer(address); err != nil {
			logger.Error("failed to start blocktx server", slog.String("err", err.Error()))
		}
	}()

	go func() {
		err = StartHealthServerBlocktx(blockTxServer)
		if err != nil {
			logger.Error("failed to start health server", slog.String("err", err.Error()))
		}
	}()

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

		if tp != nil {
			err = tp.Shutdown(context.Background())
			if err != nil {
				logger.Error("Failed to shutdown tracing provider", slog.String("err", err.Error()))
			}
		}
	}, nil
}

func StartHealthServerBlocktx(serv *blocktx.Server) error {
	gs := grpc.NewServer()
	defer gs.Stop()

	grpc_health_v1.RegisterHealthServer(gs, serv) // registration
	// register your own services
	reflection.Register(gs)

	address, err := cfg.GetString("blocktx.healthServerDialAddr") //"localhost:8005"
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

func NewBlocktxStore(dbMode string, logger *slog.Logger) (s store.BlocktxStore, err error) {
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
		s, err = postgresql.New(dbInfo, idleConns, maxOpenConns)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbMode)
	}

	return s, err
}
