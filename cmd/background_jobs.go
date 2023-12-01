package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/background_worker"
	"github.com/bitcoin-sv/arc/background_worker/jobs"
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/go-co-op/gocron"
	"github.com/spf13/viper"
)

func StartBackGroundWorker(logger *slog.Logger) (func(), error) {
	dbHost := viper.GetString("cleanBlocks.host")
	if dbHost == "" {
		return nil, errors.New("setting cleanBlocks.host not found")
	}

	dbPort := viper.GetInt("cleanBlocks.port")
	if dbPort == 0 {
		return nil, errors.New("setting cleanBlocks.port not found")
	}

	dbUsername := viper.GetString("cleanBlocks.username")
	if dbUsername == "" {
		return nil, errors.New("setting cleanBlocks.username not found")
	}

	dbPassword := viper.GetString("cleanBlocks.password")
	if dbPassword == "" {
		return nil, errors.New("setting cleanBlocks.password not found")
	}

	dbName := viper.GetString("cleanBlocks.dbName")
	if dbName == "" {
		return nil, errors.New("setting cleanBlocks.dbName not found")
	}

	dbScheme := viper.GetString("cleanBlocks.scheme")
	if dbScheme == "" {
		return nil, errors.New("setting cleanBlocks.scheme not found")
	}

	cleanBlocksRecordRetentionDays := viper.GetInt("cleanBlocks.recordRetentionDays")
	if cleanBlocksRecordRetentionDays == 0 {
		return nil, errors.New("setting cleanBlocks.recordRetentionDays not found")
	}

	intervalInHours := viper.GetInt("cleanBlocks.executionIntervalInHours")
	if intervalInHours == 0 {
		return nil, errors.New("setting cleanBlocks.executionIntervalInHours not found")
	}

	params := jobs.ClearRecrodsParams{
		DBConnectionParams: dbconn.DBConnectionParams{
			Host:     dbHost,
			Port:     dbPort,
			Username: dbUsername,
			Password: dbPassword,
			DBName:   dbName,
			Scheme:   dbScheme,
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
		logger.Info("Shutting down K8s watcher")
		sched.Shutdown()
	}, nil
}
