package metamorph

import (
	"log/slog"
	"time"
)

func WithProcessExpiredSeenTxsInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.processExpiredSeenTxsInterval = d
	}
}

func WithCacheExpiryTime(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.mapExpiryTime = d
	}
}

func WithProcessorLogger(l *slog.Logger) func(*Processor) {
	return func(p *Processor) {
		p.logger = l.With(slog.String("service", "mtm"))
	}
}

func WithLogFilePath(errLogFilePath string) func(*Processor) {
	return func(p *Processor) {
		p.logFile = errLogFilePath
	}
}

func WithNow(nowFunc func() time.Time) func(*Processor) {
	return func(p *Processor) {
		p.now = nowFunc
	}
}

func WithProcessExpiredTxsInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.processExpiredTxsTicker = time.NewTicker(d)
	}
}

func WithDataRetentionPeriod(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.dataRetentionPeriod = d
	}
}
