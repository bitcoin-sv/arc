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
	dbHost, err := config.GetString("blocktx.db.host")
	if err != nil {
		return nil, err
	}

	dbPort, err := config.GetInt("blocktx.db.port")
	if err != nil {
		return nil, err
	}

	dbUsername, err := config.GetString("blocktx.db.user")
	if err != nil {
		return nil, err
	}

	dbPassword, err := config.GetString("blocktx.db.password")
	if err != nil {
		return nil, err
	}

	dbName, err := config.GetString("blocktx.db.name")
	if err != nil {
		return nil, err
	}

	cleanBlocksRecordRetentionDays, err := config.GetInt("blocktx.cleanData.recordRetentionDays")
	if err != nil {
		return nil, err
	}

	intervalInHours, err := config.GetInt("blocktx.cleanData.executionIntervalHours")
	if err != nil {
		return nil, err
	}

	params := jobs.ClearRecrodsParams{
		DBConnectionParams: dbconn.DBConnectionParams{
			Host:     dbHost,
			Port:     dbPort,
			Username: dbUsername,
			Password: dbPassword,
			DBName:   dbName,
			Scheme:   dbSchema,
		},
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
