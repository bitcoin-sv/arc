package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/bitcoin-sv/arc/background_jobs/jobs"
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/go-co-op/gocron"
	"github.com/spf13/viper"
)

type ARCScheduler struct {
	s               *gocron.Scheduler
	IntervalInHours int
	Params          jobs.ClearRecrodsParams
}

func (sched *ARCScheduler) RunJob(table string, job func(params jobs.ClearRecrodsParams) error) {
	_, err := sched.s.Every(sched.IntervalInHours).Hours().Do(func() {
		jobs.Log(jobs.INFO, fmt.Sprintf("Clearing expired %s...", table))
		err := job(sched.Params)
		if err != nil {
			jobs.Log(jobs.ERROR, fmt.Sprintf("unable to clear expired %s %s", table, err))
			return
		}
		jobs.Log(jobs.INFO, fmt.Sprintf("%s cleanup complete", table))
	})

	if err != nil {
		jobs.Log(jobs.ERROR, fmt.Sprintf("unable to run %s job", table))
		return
	}
}

func (sched *ARCScheduler) Start() {
	sched.s.StartBlocking()
}

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
			Host:     viper.GetString("CleanBlocks.Host"),
			Port:     viper.GetInt("CleanBlocks.Port"),
			Username: viper.GetString("CleanBlocks.Username"),
			Password: viper.GetString("CleanBlocks.Password"),
			DBName:   viper.GetString("CleanBlocks.DBName"),
			Scheme:   viper.GetString("CleanBlocks.Scheme"),
		},
		RecordRetentionDays: viper.GetInt("CleanBlocks.RecordRetentionDays"),
	}

	jobs.Log(jobs.INFO, fmt.Sprintf("starting with %#v", params))

	intervalInHours := viper.GetInt("CleanBlocks.ExecutionIntervalInHours")

	sched := ARCScheduler{
		s:               gocron.NewScheduler(time.UTC),
		IntervalInHours: intervalInHours,
		Params:          params,
	}

	sched.RunJob("blocks", jobs.ClearBlocks)
	sched.RunJob("transactions", jobs.ClearTransactions)
	sched.RunJob("block transactoins map", jobs.ClearBlockTransactionsMap)

	sched.Start()
}
