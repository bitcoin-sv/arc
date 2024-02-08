package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/async/nats_mq"
	"github.com/bitcoin-sv/arc/config"
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

	txChannel := make(chan []byte, 20)
	consumer, err := nats_mq.NewNatsMQConsumer(txChannel, logger)
	if err != nil {
		return nil, err
	}

	err = consumer.ConsumeTransactions(context.Background())
	if err != nil {
		return nil, err
	}

	peerHandler, err := blocktx.NewPeerHandler(logger, blockStore, startingBlockHeight, peerURLs, network, blocktx.WithRetentionDays(recordRetentionDays), blocktx.WithTxChan(txChannel))
	if err != nil {
		return nil, err
	}

	blockTxServer := blocktx.NewServer(blockStore, logger)

	address, err := config.GetString("blocktx.listenAddr")
	if err != nil {
		return nil, err
	}
	go func() {
		if err = blockTxServer.StartGRPCServer(address); err != nil {
			logger.Error("failed to start blocktx server", slog.String("err", err.Error()))
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
		err = blockStore.Close()
		if err != nil {
			logger.Error("Error closing blocktx store", slog.String("err", err.Error()))
		}
		primaryTicker.Stop()
		peerHandler.Shutdown()
	}, nil
}
