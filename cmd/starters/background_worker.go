package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/background_worker"
	"github.com/bitcoin-sv/arc/internal/background_worker/jobs"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	cfg "github.com/bitcoin-sv/arc/internal/helpers"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
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

	metamorphAddress, err := cfg.GetString("metamorph.dialAddr")
	if err != nil {
		return nil, err
	}

	grpcMessageSize, err := cfg.GetInt("grpcMessageSize")
	if err != nil {
		return nil, err
	}

	conn, err := metamorph.DialGRPC(metamorphAddress, grpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	metamorphClient := metamorph.NewClient(metamorph_api.NewMetaMorphAPIClient(conn))

	metamorphClearDataRetentionDays, err := cfg.GetInt("backgroundWorker.metamorph.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	executionIntervalHours, err := cfg.GetInt("backgroundWorker.metamorph.executionIntervalHours")
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
	cleanBlocksRecordRetentionDays, err := cfg.GetInt("backgroundWorker.blocktx.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	executionIntervalHours, err := cfg.GetInt("backgroundWorker.blocktx.executionIntervalHours")
	if err != nil {
		return nil, err
	}

	blocktxAddress, err := cfg.GetString("blocktx.dialAddr")
	if err != nil {
		return nil, err
	}

	conn, err := blocktx.DialGRPC(blocktxAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	blocktxJobs := jobs.NewBlocktx(blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(conn)), int32(cleanBlocksRecordRetentionDays), logger)

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
