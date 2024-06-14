package logger

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

func GetSlogLevel(logLevel string) (slog.Level, error) {
	switch logLevel {
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	}

	return slog.LevelInfo, fmt.Errorf("invalid log level: %s", logLevel)
}

func NewLogger(logLevel, logFormat string) (*slog.Logger, error) {
	slogLevel, err := GetSlogLevel(logLevel)
	if err != nil {
		return nil, err
	}

	switch logFormat {
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})), nil
	case "text":
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})), nil
	case "tint":
		return slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slogLevel})), nil
	}

	return nil, fmt.Errorf("invalid log format: %s", logFormat)
}
