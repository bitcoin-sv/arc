package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/bitcoin-sv/arc/background_jobs/jobs"
	. "github.com/bitcoin-sv/arc/background_jobs/jobs"
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
			Log(ERROR, "config file not found")
			return
		} else {
			Log(ERROR, "failed to read config file")
			return
		}
	}

	params := ClearRecrodsParams{
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

	Log(INFO, fmt.Sprintf("starting with %#v", params))
	s := gocron.NewScheduler(time.UTC)

	intervalInHours := viper.GetInt("CleanBlocks.ExecutionIntervalInHours")

	_, err := s.Every(intervalInHours).Hours().Do(func() {
		Log(INFO, "Clearing expired blocks...")
		err := jobs.ClearBlocks(params)
		if err != nil {
			Log(ERROR, fmt.Sprintf("unable to clear expired blocks %s", err))
			return
		}
		Log(INFO, "Blocks cleanup complete")
	})

	if err != nil {
		Log(ERROR, "unable to run ClearBlocks job")
		return
	}

	_, err = s.Every(intervalInHours).Hours().Do(func() {
		Log(INFO, "Clearing expired transactions...")
		err := ClearTransactions(params)
		if err != nil {
			Log(ERROR, fmt.Sprintf("unable to clear expired transactions %s", err))
			return
		}
		Log(INFO, "transactions cleanup complete")
	})

	if err != nil {
		Log(ERROR, "unable to run ClearTransactions job")
		return
	}

	_, err = s.Every(intervalInHours).Hours().Do(func() {
		Log(INFO, "Clearing expired block transaction maps...")
		err := ClearBlockTransactionsMap(params)
		if err != nil {
			Log(ERROR, fmt.Sprintf("unable to clear expired block transaction maps %s", err))
			return
		}
		Log(INFO, "block transaction maps cleanup complete")
	})

	if err != nil {
		Log(ERROR, "unable to run ClearBlockTransactionsMap job")
		return
	}

	s.StartBlocking()
}
