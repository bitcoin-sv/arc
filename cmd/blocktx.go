package cmd

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/async/nats_mq"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/blocktx/store/sqlite"
	"github.com/bitcoin-sv/arc/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const BecomePrimaryintervalSecs = 30

func StartBlockTx(logger *slog.Logger) (func(), error) {
	dbMode, err := config.GetString("blocktx.db.mode")
	if err != nil {
		return nil, err
	}

	// dbMode can be sqlite, sqlite_memory or postgres
	blockStore, err := NewBlocktxStore(dbMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocktx store: %v", err)
	}

	startingBlockHeight, err := config.GetInt("blocktx.startingBlockHeight")
	if err != nil {
		return nil, err
	}

	recordRetentionDays, err := config.GetInt("blocktx.db.cleanData.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	peerSettings, err := config.GetPeerSettings()
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
	network, err := config.GetNetwork()
	if err != nil {
		return nil, err
	}

	natsURL, err := config.GetString("queueURL")
	if err != nil {
		return nil, err
	}

	registerTxInterval, err := config.GetDuration("blocktx.registerTxsInterval")
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

	natsClient, err := nats_mq.NewNatsClient(natsURL)
	if err != nil {
		return nil, err
	}

	maxBatchSize, err := config.GetInt("blocktx.mq.txsMinedMaxBatchSize")
	if err != nil {
		return nil, err
	}

	mqClient := nats_mq.NewNatsMQClient(natsClient, txChannel, nats_mq.WithMaxBatchSize(maxBatchSize))
	if err != nil {
		return nil, err
	}

	err = mqClient.SubscribeRegisterTxs()
	if err != nil {
		return nil, err
	}

	peerHandler, err := blocktx.NewPeerHandler(logger, blockStore, startingBlockHeight, peerURLs, network,
		blocktx.WithRetentionDays(recordRetentionDays),
		blocktx.WithTxChan(txChannel),
		blocktx.WithRegisterTxsInterval(registerTxInterval),
		blocktx.WithMessageQueueClient(mqClient))

	if err != nil {
		return nil, err
	}

	blockTxServer := blocktx.NewServer(blockStore, logger, peerHandler)

	address, err := config.GetString("blocktx.listenAddr")
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
			logger.Error("failed to shutdown mqClient", slog.String("err", err.Error()))
		}

		err = blockStore.Close()
		if err != nil {
			logger.Error("Error closing blocktx store", slog.String("err", err.Error()))
		}
		peerHandler.Shutdown()
	}, nil
}

func StartHealthServerBlocktx(serv *blocktx.Server) error {
	gs := grpc.NewServer()
	defer gs.Stop()

	grpc_health_v1.RegisterHealthServer(gs, serv) // registration
	// register your own services
	reflection.Register(gs)

	address, err := config.GetString("blocktx.healthServerDialAddr") //"localhost:8005"
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

func NewBlocktxStore(dbMode string) (s store.BlocktxStore, err error) {
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
		s, err = postgresql.New(dbInfo, idleConns, maxOpenConns)
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
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbMode)
	}

	return s, err
}
