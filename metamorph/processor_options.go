package metamorph

import (
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/async"
)

func WithProcessCheckIfMinedInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.processCheckIfMinedInterval = d
	}
}

func WithCacheExpiryTime(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.mapExpiryTime = d
	}
}

func WithProcessorLogger(l *slog.Logger) func(*Processor) {
	return func(p *Processor) {
		p.logger = l
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

func WithMaxMonitoredTxs(m int64) func(processor *Processor) {
	return func(p *Processor) {
		p.maxMonitoredTxs = m
	}
}

func WithPublisher(publisher async.Publisher) func(processor *Processor) {
	return func(p *Processor) {
		p.publisher = publisher
	}
}
