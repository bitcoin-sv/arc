package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/background_worker"
	"github.com/bitcoin-sv/arc/background_worker/jobs"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/dbconn"
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

	conn, err := transactionHandler.DialGRPC(metamorphAddress, grpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	meatmorphClearDataRetentionDays, err := config.GetInt("metamorph.db.cleanData.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	executionIntervalHours, err := config.GetInt("metamorph.db.cleanData.executionIntervalHours")
	if err != nil {
		return nil, err
	}

	metamorphJobs := jobs.NewMetamorph(metamorph_api.NewMetaMorphAPIClient(conn), int32(meatmorphClearDataRetentionDays), logger)

	scheduler := background_worker.NewMetamorphScheduler(gocron.NewScheduler(time.UTC), logger, executionIntervalHours)

	scheduler.RunJob("clear metamorph transactions", metamorphJobs.ClearTransactions)

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

	scheduler := background_worker.NewBlocktxScheduler(gocron.NewScheduler(time.UTC), intervalInHours, params, logger)

	clearJob := jobs.NewClearJob(logger)

	scheduler.RunJob("clear blocktx blocks", "blocks", clearJob.ClearBlocktxTable)
	scheduler.RunJob("clear blocktx transactions", "transactions", clearJob.ClearBlocktxTable)
	scheduler.RunJob("clear blocktx block transactions map", "block_transactions_map", clearJob.ClearBlocktxTable)

	scheduler.Start()

	return func() {
		logger.Info("Shutting down blocktx scheduler")
		scheduler.Shutdown()
	}, nil
}
