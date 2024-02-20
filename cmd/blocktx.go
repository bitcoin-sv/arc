package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/async/nats_mq"
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
	blockStore, err := blocktx.NewStore(dbMode)
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

	primaryTicker := time.NewTicker(time.Second * BecomePrimaryintervalSecs)
	hostName, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %v", err)
	}
	go func() {
		for range primaryTicker.C {
			if err := blockStore.TryToBecomePrimary(context.Background(), hostName); err != nil {
				logger.Error("failed to try to become primary", slog.String("err", err.Error()))
			}
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
		primaryTicker.Stop()
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
