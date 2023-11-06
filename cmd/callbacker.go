package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/callbacker"
	"github.com/bitcoin-sv/arc/config"
	"github.com/spf13/viper"
)

func StartCallbacker(logger *slog.Logger) (func(), error) {
	folder := viper.GetString("dataFolder")
	if folder == "" {
		return nil, errors.New("dataFolder not found in config")
	}

	callbackStore, err := callbacker.NewStore(folder)
	if err != nil {
		return nil, fmt.Errorf("failed to create callbacker store: %v", err)
	}

	callbackerLogger, err := config.NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to create new callbacker logger: %v", err)
	}

	var callbackWorker *callbacker.Callbacker
	callbackWorker, err = callbacker.New(callbackStore, callbacker.WithLogger(callbackerLogger))
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
