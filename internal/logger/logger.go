package logger

import (
	"context"
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

const (
	EventIDField = "arc-event"
)

func NewLogger(logLevel, logFormat string) (*slog.Logger, error) {
	slogLevel, err := getSlogLevel(logLevel)
	if err != nil {
		return nil, err
	}

	switch logFormat {
	case "json":
		return slog.New(&ArcContextHandler{slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})}), nil
	case "text":
		return slog.New(&ArcContextHandler{slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})}), nil
	case "tint":
		return slog.New(&ArcContextHandler{tint.NewHandler(os.Stdout, &tint.Options{Level: slogLevel})}), nil
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
	case "TRACE":
		return slog.LevelDebug - 4, nil // simulate trace level
	}

	return slog.LevelInfo, errors.Join(ErrLoggerInvalidLogLevel, fmt.Errorf("log level: %s", logLevel))
}

type ArcContextHandler struct {
	slog.Handler
}

func (h ArcContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if eventID, ok := ctx.Value(EventIDField).(string); ok {
		r.AddAttrs(slog.String("event-id", eventID))
	}

	return h.Handler.Handle(ctx, r)
}

func (h ArcContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ArcContextHandler{h.Handler.WithAttrs(attrs)}
}

func (h ArcContextHandler) WithGroup(name string) slog.Handler {
	return ArcContextHandler{h.Handler.WithGroup(name)}
}
