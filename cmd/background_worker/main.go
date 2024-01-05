package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/bitcoin-sv/arc/background_worker"
	"github.com/bitcoin-sv/arc/background_worker/jobs"
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/go-co-op/gocron"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigFile("config.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		if errors.Is(err, viper.ConfigFileNotFoundError{}) {
			jobs.Log(jobs.ERROR, "config file not found")
			return
		} else {
			jobs.Log(jobs.ERROR, "failed to read config file")
			return
		}
	}

	params := jobs.ClearRecordsParams{
		DBConnectionParams: dbconn.New(
			viper.GetString("cleanBlocks.host"),
			viper.GetInt("cleanBlocks.port"),
			viper.GetString("cleanBlocks.username"),
			viper.GetString("cleanBlocks.password"),
			viper.GetString("cleanBlocks.dbName"),
			viper.GetString("cleanBlocks.scheme"),
			viper.GetString("cleanBlocks.sslMode"),
		),
		RecordRetentionDays: viper.GetInt("cleanBlocks.recordRetentionDays"),
	}

	jobs.Log(jobs.INFO, fmt.Sprintf("starting with %#v", params))

	intervalInHours := viper.GetInt("cleanBlocks.executionIntervalHours")

	sched := background_worker.ARCScheduler{
		Scheduler:       gocron.NewScheduler(time.UTC),
		IntervalInHours: intervalInHours,
		Params:          params,
	}

	clearJob := jobs.NewClearJob()

	sched.RunJob("blocks", clearJob.ClearBlocks)
	sched.RunJob("transactions", clearJob.ClearTransactions)
	sched.RunJob("block transactions map", clearJob.ClearBlockTransactionsMap)

	sched.Start()
}
