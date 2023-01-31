package cmd

import (
	"context"

	"github.com/TAAL-GmbH/arc/callbacker"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

func StartCallbacker(logger utils.Logger) (func(), error) {
	folder, _ := gocore.Config().Get("dataFolder", "data")

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
