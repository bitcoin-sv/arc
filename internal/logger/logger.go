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

	LevelTrace   = slog.LevelDebug - 4
	LevelDebug   = slog.LevelDebug
	LevelInfo    = slog.LevelInfo
	LevelWarning = slog.LevelWarn
	LevelError   = slog.LevelError
)

func getSlogLevel(logLevel string) (slog.Level, error) {
	switch logLevel {
	case "INFO":
		return LevelInfo, nil
	case "WARN":
		return LevelWarning, nil
	case "ERROR":
		return LevelError, nil
	case "DEBUG":
		return LevelDebug, nil
	case "TRACE":
		return LevelTrace, nil // simulate trace level
	}

	return 0, errors.Join(ErrLoggerInvalidLogLevel, fmt.Errorf("log level: %s", logLevel))
}

func NewLogger(logLevel, logFormat string) (*slog.Logger, error) {
	slogLevel, err := getSlogLevel(logLevel)
	if err != nil {
		return nil, err
	}

	switch logFormat {
	case "json":
		return slog.New(&ArcContextHandler{slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slogLevel,
			//ReplaceAttr: replaceAttr,
		},
		)}), nil
	case "text":
		return slog.New(&ArcContextHandler{slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slogLevel,
			//ReplaceAttr: replaceAttr,
		})}), nil
	case "tint":
		return slog.New(&ArcContextHandler{tint.NewHandler(os.Stdout, &tint.Options{
			Level:       slogLevel,
			ReplaceAttr: replaceAttr,
		})}), nil
	}

	return nil, errors.Join(ErrLoggerInvalidLogFormat, fmt.Errorf("log format: %s", logFormat))
}

// replaceAttr inspired by https://go.dev/src/log/slog/example_custom_levels_test.go
func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	// Customize the name of the level key
	if a.Key == slog.LevelKey {
		level := a.Value.Any().(slog.Level) //nolint:errcheck,revive
		if level == LevelTrace {
			a.Value = slog.StringValue("TRACE")
		}
	}
	return a
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
