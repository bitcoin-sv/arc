package blocktx

import (
	"errors"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	statCollectionIntervalDefault = 60 * time.Second
)

var ErrFailedToRegisterStats = errors.New("failed to register stats collector")

type processorStats struct {
	mu                    sync.RWMutex
	currentNumOfBlockGaps prometheus.Gauge
}

func newProcessorStats(opts ...func(stats *processorStats)) *processorStats {
	p := &processorStats{
		currentNumOfBlockGaps: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_block_gaps_count",
			Help: "Current number of block gaps",
		}),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *Processor) StartCollectStats() error {
	ticker := time.NewTicker(p.statCollectionInterval)

	err := registerStats(
		p.stats.currentNumOfBlockGaps,
	)
	if err != nil {
		return err
	}

	p.waitGroup.Add(1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("Recovered from panic", "panic", r, slog.String("stacktrace", string(debug.Stack())))
			}
		}()
		defer func() {
			unregisterStats(
				p.stats.currentNumOfBlockGaps,
			)
			p.waitGroup.Done()
		}()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				collectedStats, err := p.store.GetStats(p.ctx)
				if err != nil {
					p.logger.Error("failed to get stats", slog.String("err", err.Error()))
					continue
				}

				p.stats.mu.Lock()
				p.stats.currentNumOfBlockGaps.Set(float64(collectedStats.CurrentNumOfBlockGaps))
				p.stats.mu.Unlock()
			}
		}
	}()

	return nil
}

func registerStats(cs ...prometheus.Collector) error {
	for _, c := range cs {
		err := prometheus.Register(c)
		if err != nil {
			return errors.Join(ErrFailedToRegisterStats, err)
		}
	}

	return nil
}

func unregisterStats(cs ...prometheus.Collector) {
	for _, c := range cs {
		_ = prometheus.Unregister(c)
	}
}
