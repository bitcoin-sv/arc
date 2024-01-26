package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/callbacker"
	"github.com/bitcoin-sv/arc/config"
	"github.com/spf13/viper"
)

func StartCallbacker(logger *slog.Logger) (func(), error) {
	logger.With(slog.String("service", "clb"))
	folder := viper.GetString("dataFolder")
	if folder == "" {
		return nil, errors.New("dataFolder not found in config")
	}

	callbackerExpiryInterval := viper.GetDuration("callbacker.expiryInterval")

	callbackStore, err := callbacker.NewStore(folder, callbackerExpiryInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to create callbacker store: %v", err)
	}

	callbackerInterval := viper.GetDuration("callbacker.interval")

	blocktxAddress, err := config.GetString("blocktx.dialAddr")
	if err != nil {
		return nil, err
	}

	conn, err := blocktx.DialGRPC(blocktxAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	btx := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(conn))

	var callbackWorker *callbacker.Callbacker
	callbackWorker, err = callbacker.New(callbackStore, btx, callbacker.WithLogger(logger), callbacker.WithSendCallbacksInterval(callbackerInterval))
	if err != nil {
		return nil, fmt.Errorf("failed to create callbacker: %v", err)
	}
	callbackWorker.Start()

	srv := callbacker.NewServer(logger, callbackWorker)

	address := viper.GetString("callbacker.listenAddr")
	if address == "" {
		return nil, errors.New("no callbacker.listenAddr setting found")
	}

	go func() {
		if err = srv.StartGRPCServer(address); err != nil {
			logger.Error("Could not start callbacker server", slog.String("err", err.Error()))
		}
	}()

	return func() {
		logger.Info("Shutting down callbacker store")
		err = callbackStore.Close(context.Background())
		if err != nil {
			logger.Error("Error closing callbacker store", slog.String("err", err.Error()))
		}
	}, nil
}
