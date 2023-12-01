package jobs

import (
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("module", "jobs")
}

const (
	INFO  = "INFO"
	DEBUG = "DEBUG"
	ERROR = "ERROR"
)

func Log(level, message string) {
	var logfun = map[string]func(string, ...any){
		INFO:  logger.Info,
		DEBUG: logger.Debug,
		ERROR: logger.Error,
	}
	logfun[level](message)
}
