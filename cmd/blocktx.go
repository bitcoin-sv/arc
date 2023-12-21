package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/config"
	"github.com/spf13/viper"
)

const BecomePrimaryintervalSecs = 30

func StartBlockTx(logger *slog.Logger) (func(), error) {
	dbMode := viper.GetString("blocktx.db.mode")
	if dbMode == "" {
		return nil, errors.New("blocktx.db.mode not found in config")
	}

	// dbMode can be sqlite, sqlite_memory or postgres
	blockStore, err := blocktx.NewStore(dbMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocktx store: %v", err)
	}

	blockCh := make(chan *blocktx_api.Block)

	startingBlockHeight, err := config.GetInt("blocktx.startingBlockHeight")
	if err != nil {
		return nil, err
	}

	peerHandler := blocktx.NewPeerHandler(logger, blockStore, blockCh, startingBlockHeight)

	network, err := config.GetNetwork()
	if err != nil {
		return nil, err
	}

	peerSettings, err := config.GetPeerSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get peer settings: %v", err)
	}
	blockNotifier, err := blocktx.NewBlockNotifier(blockStore, logger, blockCh, peerHandler, peerSettings, network)
	if err != nil {
		return nil, fmt.Errorf("failed to create block notifier: %v", err)
	}

	blockTxServer := blocktx.NewServer(blockStore, blockNotifier, logger)

	go func() {
		if err = blockTxServer.StartGRPCServer(); err != nil {
			logger.Error("failed to start blocktx server", slog.String("err", err.Error()))
		}
	}()

	primaryTicker := time.NewTicker(time.Second * BecomePrimaryintervalSecs)
	hostName, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %v", err)
	}
	go func() {
		for {
			select {
			case <-primaryTicker.C:
				if err := blockStore.TryToBecomePrimary(context.Background(), hostName); err != nil {
					logger.Error("failed to try to become primary", slog.String("err", err.Error()))
				}
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
		blockNotifier.Shutdown()
	}, nil
}
