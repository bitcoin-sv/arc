package main

import (
	"errors"
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
		if errors.Is(err, viper.ConfigFileNotFoundError{}) {
			logger.Fatal().Err(err).Msg("config file not found")
		} else {
			logger.Fatal().Err(err).Msg("failed to read config file")
		}
	}

	params := jobs.ClearBlockParams{
		DBConnectionParams: dbconn.DBConnectionParams{
			Host:     viper.GetString("CleanBlocks.Host"),
			Port:     viper.GetInt("CleanBlocks.Port"),
			Username: viper.GetString("CleanBlocks.Username"),
			Password: viper.GetString("CleanBlocks.Password"),
			DBName:   viper.GetString("CleanBlocks.DBName"),
			Scheme:   viper.GetString("CleanBlocks.Scheme"),
		},
		BlockRetentionDays: viper.GetInt("CleanBlocks.BlocksRetentionDays"),
	}

	logger.Info().Interface("params", params).Msg("")
	s := gocron.NewScheduler(time.UTC)

	intervalInHours := viper.GetInt("CleanBlocks.ExecutionIntervalInHours")
	_, err := s.Every(intervalInHours).Hours().Do(func() {
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
