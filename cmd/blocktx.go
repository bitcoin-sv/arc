package cmd

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/libsv/go-p2p/wire"
	"github.com/spf13/viper"
)

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

	networkStr := viper.GetString("network")
	if networkStr == "" {
		return nil, errors.New("bitcoin_network must be set")
	}

	var network wire.BitcoinNet

	switch networkStr {
	case "mainnet":
		network = wire.MainNet
	case "testnet":
		network = wire.TestNet3
	case "regtest":
		network = wire.TestNet
	default:
		return nil, fmt.Errorf("unknown bitcoin_network: %s", networkStr)
	}

	peerSettings, err := blocktx.GetPeerSettings()
	if err != nil {
		return nil, fmt.Errorf("error getting peer settings: %v", err)
	}

	blockNotifier, err := blocktx.NewBlockNotifier(blockStore, network, peerSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create block notifier: %v", err)
	}

	blockTxServer := blocktx.NewServer(blockStore, blockNotifier, logger)
	address := viper.GetString("blocktx.listenAddr")
	if address == "" {
		return nil, errors.New("no blocktx.listenAddr setting found")
	}

	prometheusEndpoint := viper.GetString("prometheusEndpoint")
	go func() {
		if err = blockTxServer.StartGRPCServer(address, prometheusEndpoint); err != nil {
			logger.Error("failed to start blocktx grpc server", slog.String("err", err.Error()))
		}
	}()

	return func() {
		logger.Info("Shutting down blocktx store")
		err = blockStore.Close()
		if err != nil {
			logger.Error("Error closing blocktx store", slog.String("err", err.Error()))
		}

		blockTxServer.Shutdown()
	}, nil
}
