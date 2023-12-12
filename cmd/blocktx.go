package cmd

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/blocktx"
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
		logger.Fatalf("Error creating blocktx store: %v", err)
	}

	blockTxServer := blocktx.NewServer(blockStore, logger)

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
	}, nil
}
