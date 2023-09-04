package cmd

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/callbacker"
	"github.com/ordishs/go-utils"
	"github.com/spf13/viper"
)

func StartCallbacker(logger utils.Logger) (func(), error) {
	folder := viper.GetString("dataFolder")
	if folder == "" {
		return nil, errors.New("dataFolder not found in config")
	}

	callbackStore, err := callbacker.NewStore(folder)
	if err != nil {
		logger.Fatalf("Error creating callbacker store: %v", err)
	}

	var callbackWorker *callbacker.Callbacker
	callbackWorker, err = callbacker.New(callbackStore)
	if err != nil {
		logger.Fatalf("Could not create callbacker: %v", err)
	}
	callbackWorker.Start()

	srv := callbacker.NewServer(logger, callbackWorker)

	go func() {
		if err = srv.StartGRPCServer(); err != nil {
			logger.Fatalf("Could not start callbacker server: %v", err)
		}
	}()

	return func() {
		logger.Infof("Shutting down callbacker store")
		err = callbackStore.Close(context.Background())
		if err != nil {
			logger.Errorf("Error closing callbacker store: %v", err)
		}
	}, nil
}
