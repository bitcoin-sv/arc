package cmd

import (
	"path"
	"path/filepath"
	"time"

	"github.com/TAAL-GmbH/arc/callbacker"
	callbackerBadgerhold "github.com/TAAL-GmbH/arc/callbacker/store/badgerhold"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

func StartCallbacker(logger utils.Logger) {
	folder, _ := gocore.Config().Get("dataFolder", "data")

	f, err := filepath.Abs(path.Join(folder, "callbacker"))
	if err != nil {
		logger.Fatalf("Could not get absolute path: %v", err)
	}

	callbackStore, err := callbackerBadgerhold.New(f, 2*time.Minute)
	if err != nil {
		logger.Fatalf("Could not open callbacker store: %v", err)
	}

	var callbackWorker *callbacker.Callbacker
	callbackWorker, err = callbacker.NewCallbacker(callbackStore)
	if err != nil {
		logger.Fatalf("Could not create callbacker: %v", err)
	}
	callbackWorker.Start()

	srv := callbacker.NewServer(logger, callbackWorker)
	if err = srv.StartGRPCServer(); err != nil {
		logger.Fatalf("Could not start callbacker server: %v", err)
	}
}
