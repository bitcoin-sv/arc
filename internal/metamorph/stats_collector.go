package metamorph

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	statCollectionIntervalDefault = 60 * time.Second
)

type processorStats struct {
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
}

func newProcessorStats() *processorStats {
	c := &processorStats{
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
	}

	return c
}

func (p *Processor) StartCollectStats() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelCollectStats = cancel
	p.quitCollectStatsComplete = make(chan struct{})

	ticker := time.NewTicker(statCollectionIntervalDefault)

	prometheus.MustRegister(p.stats.statusStored, p.stats.statusAnnouncedToNetwork, p.stats.statusRequestedByNetwork, p.stats.statusSentToNetwork, p.stats.statusAcceptedByNetwork, p.stats.statusSeenOnNetwork, p.stats.statusMined, p.stats.statusRejected, p.stats.statusSeenInOrphanMempool)

	go func() {
		defer func() {
			p.quitCollectStatsComplete <- struct{}{}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:

				getStatsSince := p.now().Add(-1 * p.mapExpiryTime)

				collectedStats, err := p.store.GetStats(ctx, getStatsSince)
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
				p.stats.mu.Unlock()
			}
		}

	}()
}
