package blocktx

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

const (
	statCollectionIntervalDefault = 60 * time.Second
)

var (
	ErrFailedToRegisterStats        = errors.New("failed to register stats collector")
	ErrFailedToStartCollectingStats = errors.New("failed to start collecting stats")
)

func WithStatCollectionInterval(d time.Duration) func(*StatsCollector) {
	return func(processor *StatsCollector) {
		processor.statCollectionInterval = d
	}
}

type StatsCollector struct {
	CurrentNumOfBlockGaps  prometheus.Gauge
	ConnectedPeers         prometheus.Gauge
	ReconnectingPeers      prometheus.Gauge
	statCollectionInterval time.Duration
	waitGroup              *sync.WaitGroup
	cancelAll              context.CancelFunc
	ctx                    context.Context
	logger                 *slog.Logger
	store                  store.BlocktxStore
	pm                     PeerManager
}

func NewStatsCollector(logger *slog.Logger, pm PeerManager, store store.BlocktxStore, opts ...func(stats *StatsCollector)) *StatsCollector {
	p := &StatsCollector{
		CurrentNumOfBlockGaps: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_block_gaps_count",
			Help: "Current number of block gaps",
		}),
		ConnectedPeers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_blocktx_connected_peers",
			Help: "Current number of connected peers",
		}),
		ReconnectingPeers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_blocktx_reconnecting_peers",
			Help: "Current number of peers that are reconnecting",
		}),
		statCollectionInterval: statCollectionIntervalDefault,
		waitGroup:              &sync.WaitGroup{},
		logger:                 logger,
		store:                  store,
		pm:                     pm,
	}

	for _, opt := range opts {
		opt(p)
	}
	ctx, cancelAll := context.WithCancel(context.Background())
	p.cancelAll = cancelAll
	p.ctx = ctx
	return p
}

func (p *StatsCollector) Start() error {
	ticker := time.NewTicker(p.statCollectionInterval)

	err := registerStats(
		p.CurrentNumOfBlockGaps,
		p.ConnectedPeers,
		p.ReconnectingPeers,
	)
	if err != nil {
		return errors.Join(ErrFailedToStartCollectingStats, err)
	}

	p.waitGroup.Add(1)

	go func() {
		defer func() {
			unregisterStats(
				p.CurrentNumOfBlockGaps,
				p.ConnectedPeers,
				p.ReconnectingPeers,
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

				p.CurrentNumOfBlockGaps.Set(float64(collectedStats.CurrentNumOfBlockGaps))

				// Update connected and reconnecting peers
				connectedPeers := int(p.pm.CountConnectedPeers()) // #nosec G115
				peers := p.pm.GetPeers()
				p.ConnectedPeers.Set(float64(connectedPeers))
				p.ReconnectingPeers.Set(float64(len(peers) - connectedPeers))
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

func (p *StatsCollector) Shutdown() {
	p.cancelAll()
	p.waitGroup.Wait()
}
