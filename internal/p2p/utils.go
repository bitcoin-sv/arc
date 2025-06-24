package p2p

import (
	"log/slog"
	"strings"
)

func slogUpperString(key, val string) slog.Attr {
	return slog.String(key, strings.ToUpper(val))
}

const slogLvlTrace slog.Level = slog.LevelDebug - 4
