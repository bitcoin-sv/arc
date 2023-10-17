package config

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

func GetSlogLevel() (slog.Level, error) {

	logLevelString := viper.GetString("logLevel")

	switch logLevelString {
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	}

	return slog.LevelInfo, fmt.Errorf("invalid log level: %s", logLevelString)
}
