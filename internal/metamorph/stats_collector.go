package metamorph

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	statCollectionIntervalDefault = 60 * time.Second
	notSeenLimitDefault           = 10 * time.Minute
	notMinedLimitDefault          = 20 * time.Minute
)

type ProcessorStatsCollector struct {
	notSeenLimit  time.Duration
	notMinedLimit time.Duration

	mu                        sync.RWMutex
	statusStored              prometheus.Gauge
	statusAnnouncedToNetwork  prometheus.Gauge
	statusRequestedByNetwork  prometheus.Gauge
	statusSentToNetwork       prometheus.Gauge
	statusAcceptedByNetwork   prometheus.Gauge
	statusSeenOnNetwork       prometheus.Gauge
	statusMined               prometheus.Gauge
	statusRejected            prometheus.Gauge
	statusSeenInOrphanMempool prometheus.Gauge
	statusNotMined            prometheus.Gauge
	statusNotSeen             prometheus.Gauge
	healthyPeerConnections    prometheus.Gauge
}

func WithLimits(notSeenLimit time.Duration, notMinedLimit time.Duration) func(*ProcessorStatsCollector) {
	return func(p *ProcessorStatsCollector) {
		p.notSeenLimit = notSeenLimit
		p.notMinedLimit = notMinedLimit
	}
}

func newProcessorStats(opts ...func(stats *ProcessorStatsCollector)) *ProcessorStatsCollector {
	p := &ProcessorStatsCollector{
		statusStored: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_stored_count",
			Help: "Number of monitored transactions with status STORED",
		}),
		statusAnnouncedToNetwork: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_announced_count",
			Help: "Number of monitored transactions with status ANNOUNCED_TO_NETWORK",
		}),
		statusRequestedByNetwork: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_requested_count",
			Help: "Number of monitored transactions with status REQUESTED_BY_NETWORK",
		}),
		statusSentToNetwork: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_sent_count",
			Help: "Number of monitored transactions with status SENT_TO_NETWORK",
		}),
		statusAcceptedByNetwork: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_accepted_count",
			Help: "Number of monitored transactions with status ACCEPTED_BY_NETWORK",
		}),
		statusSeenOnNetwork: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_seen_on_network_count",
			Help: "Number of monitored transactions with status SEEN_ON_NETWORK",
		}),
		statusMined: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_mined_count",
			Help: "Number of monitored transactions with status MINED",
		}),
		statusRejected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_rejected_count",
			Help: "Number of monitored transactions with status REJECTED",
		}),
		statusSeenInOrphanMempool: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_seen_in_orphan_mempool_count",
			Help: "Number of monitored transactions with status SEEN_IN_ORPHAN_MEMPOOL",
		}),
		healthyPeerConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_healthy_peers_count",
			Help: "Number of healthy peer connections",
		}),
		notSeenLimit:  notSeenLimitDefault,
		notMinedLimit: notMinedLimitDefault,
	}

	for _, opt := range opts {
		opt(p)
	}

	p.statusNotMined = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arc_status_not_mined_count",
		Help: fmt.Sprintf("Number of monitored transactions which are SEEN_ON_NETWORK but haven reached status MINED for more than %s", p.notMinedLimit.String()),
	})
	p.statusNotSeen = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arc_status_not_seen_count",
		Help: fmt.Sprintf("Number of monitored transactions which are not SEEN_ON_NETWORK for more than %s", p.notSeenLimit.String()),
	})

	return p
}

func (p *Processor) StartCollectStats() error {
	ctx, cancel := context.WithCancel(context.Background())
	p.CancelCollectStats = cancel
	p.waitGroup.Add(1)

	ticker := time.NewTicker(p.statCollectionInterval)

	err := registerStats(
		p.Stats.statusStored,
		p.Stats.statusAnnouncedToNetwork,
		p.Stats.statusRequestedByNetwork,
		p.Stats.statusSentToNetwork,
		p.Stats.statusAcceptedByNetwork,
		p.Stats.statusSeenOnNetwork,
		p.Stats.statusMined,
		p.Stats.statusRejected,
		p.Stats.statusSeenInOrphanMempool,
		p.Stats.statusNotMined,
		p.Stats.statusNotSeen,
		p.Stats.healthyPeerConnections,
	)
	if err != nil {
		p.waitGroup.Done()
		return err
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("Recovered from panic", "panic", r, slog.String("stacktrace", string(debug.Stack())))
			}
		}()
		defer p.waitGroup.Done()
		defer unregisterStats(
			p.Stats.statusStored,
			p.Stats.statusAnnouncedToNetwork,
			p.Stats.statusRequestedByNetwork,
			p.Stats.statusSentToNetwork,
			p.Stats.statusAcceptedByNetwork,
			p.Stats.statusSeenOnNetwork,
			p.Stats.statusMined,
			p.Stats.statusRejected,
			p.Stats.statusSeenInOrphanMempool,
			p.Stats.statusNotMined,
			p.Stats.statusNotSeen,
			p.Stats.healthyPeerConnections,
		)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:

				getStatsSince := p.now().Add(-1 * p.mapExpiryTime)

				collectedStats, err := p.store.GetStats(ctx, getStatsSince, p.Stats.notSeenLimit, p.Stats.notMinedLimit)
				if err != nil {
					p.logger.Error("failed to get stats", slog.String("err", err.Error()))
					continue
				}

				healthyConnections := 0

				for _, peer := range p.pm.GetPeers() {
					if peer.Connected() && peer.IsHealthy() {
						healthyConnections++
						continue
					}
				}

				p.Stats.mu.Lock()
				p.Stats.statusStored.Set(float64(collectedStats.StatusStored))
				p.Stats.statusAnnouncedToNetwork.Set(float64(collectedStats.StatusAnnouncedToNetwork))
				p.Stats.statusRequestedByNetwork.Set(float64(collectedStats.StatusRequestedByNetwork))
				p.Stats.statusSentToNetwork.Set(float64(collectedStats.StatusSentToNetwork))
				p.Stats.statusAcceptedByNetwork.Set(float64(collectedStats.StatusAcceptedByNetwork))
				p.Stats.statusSeenOnNetwork.Set(float64(collectedStats.StatusSeenOnNetwork))
				p.Stats.statusMined.Set(float64(collectedStats.StatusMined))
				p.Stats.statusRejected.Set(float64(collectedStats.StatusRejected))
				p.Stats.statusSeenInOrphanMempool.Set(float64(collectedStats.StatusSeenInOrphanMempool))
				p.Stats.statusNotMined.Set(float64(collectedStats.StatusNotMined))
				p.Stats.statusNotSeen.Set(float64(collectedStats.StatusNotSeen))
				p.Stats.healthyPeerConnections.Set(float64(healthyConnections))
				p.Stats.mu.Unlock()
			}
		}
	}()

	return nil
}

func registerStats(cs ...prometheus.Collector) error {
	for _, c := range cs {
		err := prometheus.Register(c)
		if err != nil {
			return fmt.Errorf("failed to register stats collector: %w", err)
		}
	}

	return nil
}

func unregisterStats(cs ...prometheus.Collector) {
	for _, c := range cs {
		_ = prometheus.Unregister(c)
	}
}
