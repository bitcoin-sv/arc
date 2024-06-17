package metamorph

import (
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

type processorStats struct {
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
	statusMinedTotal          prometheus.Gauge
	statusSeenOnNetworkTotal  prometheus.Gauge
	statusNotSeenStat         int64
}

func WithLimits(notSeenLimit time.Duration, notMinedLimit time.Duration) func(*processorStats) {
	return func(p *processorStats) {
		p.notSeenLimit = notSeenLimit
		p.notMinedLimit = notMinedLimit
	}
}

func newProcessorStats(opts ...func(stats *processorStats)) *processorStats {
	p := &processorStats{
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
		statusSeenOnNetworkTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_seen_on_network_total_count",
			Help: "Total number of monitored transactions with status SEEN_ON_NETWORK",
		}),
		statusMinedTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_mined_total_count",
			Help: "Total number of monitored transactions with status MINED",
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

func (p *Processor) GetStatusNotSeen() int64 {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()
	return p.stats.statusNotSeenStat
}

func (p *Processor) StartCollectStats() error {
	p.waitGroup.Add(1)

	ticker := time.NewTicker(p.statCollectionInterval)

	err := registerStats(
		p.stats.statusStored,
		p.stats.statusAnnouncedToNetwork,
		p.stats.statusRequestedByNetwork,
		p.stats.statusSentToNetwork,
		p.stats.statusAcceptedByNetwork,
		p.stats.statusSeenOnNetwork,
		p.stats.statusMined,
		p.stats.statusRejected,
		p.stats.statusSeenInOrphanMempool,
		p.stats.statusNotMined,
		p.stats.statusNotSeen,
		p.stats.statusSeenOnNetworkTotal,
		p.stats.statusMinedTotal,
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
			p.stats.statusStored,
			p.stats.statusAnnouncedToNetwork,
			p.stats.statusRequestedByNetwork,
			p.stats.statusSentToNetwork,
			p.stats.statusAcceptedByNetwork,
			p.stats.statusSeenOnNetwork,
			p.stats.statusMined,
			p.stats.statusRejected,
			p.stats.statusSeenInOrphanMempool,
			p.stats.statusNotMined,
			p.stats.statusNotSeen,
			p.stats.statusSeenOnNetworkTotal,
			p.stats.statusMinedTotal,
		)

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:

				getStatsSince := p.now().Add(-1 * p.mapExpiryTime)

				collectedStats, err := p.store.GetStats(p.ctx, getStatsSince, p.stats.notSeenLimit, p.stats.notMinedLimit)
				if err != nil {
					p.logger.Error("failed to get stats", slog.String("err", err.Error()))
					continue
				}

				p.stats.mu.Lock()
				p.stats.statusStored.Set(float64(collectedStats.StatusStored))
				p.stats.statusAnnouncedToNetwork.Set(float64(collectedStats.StatusAnnouncedToNetwork))
				p.stats.statusRequestedByNetwork.Set(float64(collectedStats.StatusRequestedByNetwork))
				p.stats.statusSentToNetwork.Set(float64(collectedStats.StatusSentToNetwork))
				p.stats.statusAcceptedByNetwork.Set(float64(collectedStats.StatusAcceptedByNetwork))
				p.stats.statusSeenOnNetwork.Set(float64(collectedStats.StatusSeenOnNetwork))
				p.stats.statusMined.Set(float64(collectedStats.StatusMined))
				p.stats.statusRejected.Set(float64(collectedStats.StatusRejected))
				p.stats.statusSeenInOrphanMempool.Set(float64(collectedStats.StatusSeenInOrphanMempool))
				p.stats.statusNotMined.Set(float64(collectedStats.StatusNotMined))
				p.stats.statusNotSeen.Set(float64(collectedStats.StatusNotSeen))
				p.stats.statusNotSeenStat = collectedStats.StatusNotSeen
				p.stats.statusSeenOnNetworkTotal.Set(float64(collectedStats.StatusSeenOnNetworkTotal))
				p.stats.statusMinedTotal.Set(float64(collectedStats.StatusMinedTotal))
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
