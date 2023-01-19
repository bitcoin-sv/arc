package cmd

import (
	"github.com/TAAL-GmbH/arc/callbacker"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

func StartCallbacker(logger utils.Logger) {
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
	if err = srv.StartGRPCServer(); err != nil {
		logger.Fatalf("Could not start callbacker server: %v", err)
	}
}
