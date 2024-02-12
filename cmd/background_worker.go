package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/background_worker"
	"github.com/bitcoin-sv/arc/background_worker/jobs"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/go-co-op/gocron"
)

func StartBackGroundWorker(logger *slog.Logger) (func(), error) {
	shutdownBlocktxScheduler, err := startBlocktxScheduler(logger.With("service", "background-worker"))
	if err != nil {
		return nil, err
	}

	shutdownMetamorphScheduler, err := startMetamorphScheduler(logger.With("service", "background-worker"))
	if err != nil {
		return nil, err
	}

	return func() {
		shutdownBlocktxScheduler()
		shutdownMetamorphScheduler()
	}, err
}

func startMetamorphScheduler(logger *slog.Logger) (func(), error) {

	metamorphAddress, err := config.GetString("metamorph.dialAddr")
	if err != nil {
		return nil, err
	}

	grpcMessageSize, err := config.GetInt("grpcMessageSize")
	if err != nil {
		return nil, err
	}

	metamorphClient, err := metamorph.NewMetamorph(metamorphAddress, grpcMessageSize)
	if err != nil {
		return nil, err
	}

	metamorphClearDataRetentionDays, err := config.GetInt("metamorph.db.cleanData.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	executionIntervalHours, err := config.GetInt("metamorph.db.cleanData.executionIntervalHours")
	if err != nil {
		return nil, err
	}

	metamorphJobs := jobs.NewMetamorph(metamorphClient, int32(metamorphClearDataRetentionDays), logger)

	scheduler := background_worker.NewScheduler(gocron.NewScheduler(time.UTC), time.Duration(executionIntervalHours)*time.Hour, logger)

	scheduler.RunJob("clear metamorph transactions", metamorphJobs.ClearTransactions)

	scheduler.Start()

	return func() {
		logger.Info("Shutting metamorph scheduler")
		scheduler.Shutdown()
	}, nil
}

func startBlocktxScheduler(logger *slog.Logger) (func(), error) {
	logger.With("service", "background-worker")
	blocktxAddress, err := config.GetString("blocktx.dialAddr")
	if err != nil {
		return nil, err
	}

	conn, err := blocktx.DialGRPC(blocktxAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	cleanBlocksRecordRetentionDays, err := config.GetInt("blocktx.db.cleanData.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	executionIntervalHours, err := config.GetInt("blocktx.db.cleanData.executionIntervalHours")
	if err != nil {
		return nil, err
	}

	blocktxJobs := jobs.NewBlocktx(blocktx_api.NewBlockTxAPIClient(conn), int32(cleanBlocksRecordRetentionDays), logger)

	scheduler := background_worker.NewScheduler(gocron.NewScheduler(time.UTC), time.Duration(executionIntervalHours)*time.Hour, logger)

	scheduler.RunJob("clear blocktx blocks", blocktxJobs.ClearBlocks)
	scheduler.RunJob("clear blocktx transactions", blocktxJobs.ClearTransactions)
	scheduler.RunJob("clear blocktx block transactions map", blocktxJobs.ClearBlockTransactionsMap)

	scheduler.Start()

	return func() {
		logger.Info("Shutting Background-Worker")
		scheduler.Shutdown()
	}, nil
}
