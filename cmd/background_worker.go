package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/background_worker"
	"github.com/bitcoin-sv/arc/background_worker/jobs"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/go-co-op/gocron"
)

const (
	dbSchema = "postgres"
)

func StartBackGroundWorker(logger *slog.Logger) (func(), error) {
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

	params := jobs.ClearRecrodsParams{
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

	jobs.Log(jobs.INFO, fmt.Sprintf("starting with %#v", params))

	sched := background_worker.ARCScheduler{
		Scheduler:       gocron.NewScheduler(time.UTC),
		IntervalInHours: intervalInHours,
		Params:          params,
	}

	sched.RunJob("blocks", jobs.ClearBlocks)
	sched.RunJob("transactions", jobs.ClearTransactions)
	sched.RunJob("block transactions map", jobs.ClearBlockTransactionsMap)

	sched.Start()

	return func() {
		logger.Info("Shutting Background-Worker")
		sched.Shutdown()
	}, nil
}
