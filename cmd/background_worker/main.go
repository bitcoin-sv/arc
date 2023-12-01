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

	params := jobs.ClearRecrodsParams{
		DBConnectionParams: dbconn.DBConnectionParams{
			Host:     viper.GetString("cleanBlocks.host"),
			Port:     viper.GetInt("cleanBlocks.port"),
			Username: viper.GetString("cleanBlocks.username"),
			Password: viper.GetString("cleanBlocks.password"),
			DBName:   viper.GetString("cleanBlocks.dbName"),
			Scheme:   viper.GetString("cleanBlocks.scheme"),
		},
		RecordRetentionDays: viper.GetInt("cleanBlocks.recordRetentionDays"),
	}

	jobs.Log(jobs.INFO, fmt.Sprintf("starting with %#v", params))

	intervalInHours := viper.GetInt("cleanBlocks.executionIntervalInHours")

	sched := background_worker.ARCScheduler{
		Scheduler:       gocron.NewScheduler(time.UTC),
		IntervalInHours: intervalInHours,
		Params:          params,
	}

	sched.RunJob("blocks", jobs.ClearBlocks)
	sched.RunJob("transactions", jobs.ClearTransactions)
	sched.RunJob("block transactions map", jobs.ClearBlockTransactionsMap)

	sched.Start()
}
