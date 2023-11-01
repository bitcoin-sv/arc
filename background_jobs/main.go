package main

import (
	"time"

	"github.com/bitcoin-sv/arc/background_jobs/jobs"
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var logger zerolog.Logger

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger = log.With().Str("scheduler", "arc-scheduler").Logger()
}

func main() {
	viper.SetConfigFile("config.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Fatal().Err(err).Msg("config file not found")
		} else {
			logger.Fatal().Err(err).Msg("failed to read config file")
		}
	}

	params := jobs.ClearBlockParams{
		DBConnectionParams: dbconn.DBConnectionParams{
			Host:     viper.GetString("Host"),
			Port:     viper.GetInt("Port"),
			Username: viper.GetString("Username"),
			Password: viper.GetString("Password"),
			DBName:   viper.GetString("DBName"),
			Scheme:   viper.GetString("Scheme"),
		},
		BlockRetentionDays: viper.GetInt("BlocksRetentionDays"),
	}

	logger.Info().Interface("params", params).Msg("")
	s := gocron.NewScheduler(time.UTC)

	_, err := s.Every(1).Minute().Do(func() {
		logger.Info().Msg("Clearing expired blocks...")
		err := jobs.ClearBlocks(params)
		if err != nil {
			logger.Error().Err(err).Msg("unable to clear expired blocks")
		}
		logger.Info().Msg("Blocks cleanup complete")
	})

	if err != nil {
		logger.Fatal().Err(err).Msg("unable to run job")
	}

	s.StartBlocking()
	
}
