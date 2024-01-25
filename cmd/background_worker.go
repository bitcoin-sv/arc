package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/background_worker"
	"github.com/bitcoin-sv/arc/background_worker/jobs"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/go-co-op/gocron"
)

const (
	dbSchema = "postgres"
)

func StartBackGroundWorker(logger *slog.Logger) (func(), error) {
	shutdownBlocktxScheduler, err := startBlocktxScheduler(logger)
	if err != nil {
		return nil, err
	}

	shutdownMetamorphScheduler, err := startMetamorphScheduler(logger)
	if err != nil {
		return nil, err
	}

	return func() {
		shutdownBlocktxScheduler()
		shutdownMetamorphScheduler()
	}, err
}

func startMetamorphScheduler(logger *slog.Logger) (func(), error) {
	logger.With("service", "background-worker")

	metamorphAddress, err := config.GetString("metamorph.dialAddr")
	if err != nil {
		return nil, err
	}
	grpcMessageSize, err := config.GetInt("grpcMessageSize")
	if err != nil {
		return nil, err
	}

	metamorphConn, err := metamorph.DialGRPC(metamorphAddress, grpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection to metamorph with address %s: %v", metamorphAddress, err)
	}

	metamorphClient := metamorph_api.NewMetaMorphAPIClient(metamorphConn)

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

	scheduler.RunJob("clear metamorph transactions", "", metamorphJobs.ClearTransactions)

	scheduler.Start()

	return func() {
		logger.Info("Shutting metamorph scheduler")
		scheduler.Shutdown()
	}, nil
}

func startBlocktxScheduler(logger *slog.Logger) (func(), error) {
	dbHost, err := config.GetString("blocktx.db.postgres.host")
	if err != nil {
		return nil, err
	}

	dbPort, err := config.GetInt("blocktx.db.postgres.port")
	if err != nil {
		return nil, err
	}

	dbUsername, err := config.GetString("blocktx.db.postgres.user")
	if err != nil {
		return nil, err
	}

	dbPassword, err := config.GetString("blocktx.db.postgres.password")
	if err != nil {
		return nil, err
	}

	dbName, err := config.GetString("blocktx.db.postgres.name")
	if err != nil {
		return nil, err
	}

	sslMode, err := config.GetString("blocktx.db.postgres.sslMode")
	if err != nil {
		return nil, err
	}

	cleanBlocksRecordRetentionDays, err := config.GetInt("blocktx.db.cleanData.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	intervalInHours, err := config.GetInt("blocktx.db.cleanData.executionIntervalHours")
	if err != nil {
		return nil, err
	}

	params := jobs.ClearRecordsParams{
		DBConnectionParams: dbconn.New(
			dbHost,
			dbPort,
			dbUsername,
			dbPassword,
			dbName,
			dbSchema,
			sslMode,
		),
		RecordRetentionDays: cleanBlocksRecordRetentionDays,
	}

	scheduler := background_worker.NewScheduler(gocron.NewScheduler(time.UTC), time.Duration(intervalInHours)*time.Hour, logger)

	clearJob := jobs.NewClearJob(logger, params)

	scheduler.RunJob("clear blocktx blocks", "blocks", clearJob.ClearBlocktxTable)
	scheduler.RunJob("clear blocktx transactions", "transactions", clearJob.ClearBlocktxTable)
	scheduler.RunJob("clear blocktx block transactions map", "block_transactions_map", clearJob.ClearBlocktxTable)

	scheduler.Start()

	return func() {
		logger.Info("Shutting down blocktx scheduler")
		scheduler.Shutdown()
	}, nil
}
