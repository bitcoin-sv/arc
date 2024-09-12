package logger

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

var (
	ErrLoggerInvalidLogLevel  = fmt.Errorf("invalid log level")
	ErrLoggerInvalidLogFormat = fmt.Errorf("invalid log format")
)

func NewLogger(logLevel, logFormat string) (*slog.Logger, error) {
	slogLevel, err := getSlogLevel(logLevel)
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

	return nil, errors.Join(ErrLoggerInvalidLogFormat, fmt.Errorf("log format: %s", logFormat))
}

func getSlogLevel(logLevel string) (slog.Level, error) {
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

	return slog.LevelInfo, errors.Join(ErrLoggerInvalidLogLevel, fmt.Errorf("log level: %s", logLevel))
}
