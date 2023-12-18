package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/config"
	"github.com/ordishs/go-utils"
	"github.com/spf13/viper"
)

const BecomePrimaryintervalSecs = 30

func StartBlockTx(logger utils.Logger) (func(), error) {
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

	peerHandler := blocktx.NewPeerHandler(logger, blockStore, blockCh)

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
			logger.Fatalf("%v", err)
		}
	}()

	go func() {
		for {
			hostName, err := os.Hostname()
			if err != nil {
				logger.Fatalf("%v", err)
			} else {
				if err := blockStore.TryToBecomePrimary(context.Background(), hostName); err != nil {
					logger.Fatalf("%v", err)
				}
			}
			time.Sleep(time.Second * BecomePrimaryintervalSecs)
		}
	}()

	return func() {
		logger.Infof("Shutting down blocktx store")
		err = blockStore.Close()
		if err != nil {
			logger.Errorf("Error closing blocktx store: %v", err)
		}
		blockNotifier.Shutdown()
	}, nil
}
