package metamorph

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	statCollectionIntervalDefault = 60 * time.Second
	notSeenLimitDefault           = 10 * time.Minute
	notFinalLimitDefault          = 20 * time.Minute
)

var ErrFailedToRegisterStats = errors.New("failed to register stats collector")

type processorStats struct {
	notSeenLimit               time.Duration
	notFinalLimit              time.Duration
	statusStored               prometheus.Gauge
	statusAnnouncedToNetwork   prometheus.Gauge
	statusRequestedByNetwork   prometheus.Gauge
	statusSentToNetwork        prometheus.Gauge
	statusAcceptedByNetwork    prometheus.Gauge
	statusSeenInOrphanMempool  prometheus.Gauge
	statusSeenOnNetwork        prometheus.Gauge
	statusDoubleSpendAttempted prometheus.Gauge
	statusRejected             prometheus.Gauge
	statusMined                prometheus.Gauge
	statusNotFinal             prometheus.Gauge
	statusNotSeen              prometheus.Gauge
	statusMinedTotal           prometheus.Gauge
	statusSeenOnNetworkTotal   prometheus.Gauge
}

func WithLimits(notSeenLimit time.Duration, notFinalLimit time.Duration) func(*processorStats) {
	return func(p *processorStats) {
		p.notSeenLimit = notSeenLimit
		p.notFinalLimit = notFinalLimit
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
		statusSeenInOrphanMempool: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_seen_in_orphan_mempool_count",
			Help: "Number of monitored transactions with status SEEN_IN_ORPHAN_MEMPOOL",
		}),
		statusSeenOnNetwork: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_seen_on_network_count",
			Help: "Number of monitored transactions with status SEEN_ON_NETWORK",
		}),
		statusDoubleSpendAttempted: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_double_spend_attempted_count",
			Help: "Number of monitored transactions with status DOUBLE_SPEND_ATTEMPTED",
		}),
		statusRejected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_rejected_count",
			Help: "Number of monitored transactions with status REJECTED",
		}),
		statusMined: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_status_mined_count",
			Help: "Number of monitored transactions with status MINED",
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
		notFinalLimit: notFinalLimitDefault,
	}

	for _, opt := range opts {
		opt(p)
	}

	p.statusNotFinal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arc_status_not_final_count",
		Help: fmt.Sprintf("Number of monitored transactions which are not in a final state of either MINED or rejected %s", p.notFinalLimit.String()),
	})
	p.statusNotSeen = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arc_status_not_seen_count",
		Help: fmt.Sprintf("Number of monitored transactions which are not SEEN_ON_NETWORK for more than %s", p.notSeenLimit.String()),
	})

	return p
}

func (p *Processor) StartCollectStats() error {
	ticker := time.NewTicker(p.statCollectionInterval)

	err := registerStats(
		p.stats.statusStored,
		p.stats.statusAnnouncedToNetwork,
		p.stats.statusRequestedByNetwork,
		p.stats.statusSentToNetwork,
		p.stats.statusAcceptedByNetwork,
		p.stats.statusSeenInOrphanMempool,
		p.stats.statusSeenOnNetwork,
		p.stats.statusDoubleSpendAttempted,
		p.stats.statusRejected,
		p.stats.statusMined,
		p.stats.statusNotFinal,
		p.stats.statusNotSeen,
		p.stats.statusSeenOnNetworkTotal,
		p.stats.statusMinedTotal,
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
				p.stats.statusStored,
				p.stats.statusAnnouncedToNetwork,
				p.stats.statusRequestedByNetwork,
				p.stats.statusSentToNetwork,
				p.stats.statusAcceptedByNetwork,
				p.stats.statusSeenInOrphanMempool,
				p.stats.statusSeenOnNetwork,
				p.stats.statusDoubleSpendAttempted,
				p.stats.statusRejected,
				p.stats.statusMined,
				p.stats.statusNotFinal,
				p.stats.statusNotSeen,
				p.stats.statusSeenOnNetworkTotal,
				p.stats.statusMinedTotal,
			)
			p.waitGroup.Done()
		}()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:

				getStatsSince := p.now().Add(-1 * p.mapExpiryTime)

				collectedStats, err := p.store.GetStats(p.ctx, getStatsSince, p.stats.notSeenLimit, p.stats.notFinalLimit)
				if err != nil {
					p.logger.Error("failed to get stats", slog.String("err", err.Error()))
					continue
				}

				p.stats.statusStored.Set(float64(collectedStats.StatusStored))
				p.stats.statusAnnouncedToNetwork.Set(float64(collectedStats.StatusAnnouncedToNetwork))
				p.stats.statusRequestedByNetwork.Set(float64(collectedStats.StatusRequestedByNetwork))
				p.stats.statusSentToNetwork.Set(float64(collectedStats.StatusSentToNetwork))
				p.stats.statusAcceptedByNetwork.Set(float64(collectedStats.StatusAcceptedByNetwork))
				p.stats.statusSeenInOrphanMempool.Set(float64(collectedStats.StatusSeenInOrphanMempool))
				p.stats.statusSeenOnNetwork.Set(float64(collectedStats.StatusSeenOnNetwork))
				p.stats.statusDoubleSpendAttempted.Set(float64(collectedStats.StatusDoubleSpendAttempted))
				p.stats.statusRejected.Set(float64(collectedStats.StatusRejected))
				p.stats.statusMined.Set(float64(collectedStats.StatusMined))
				p.stats.statusNotFinal.Set(float64(collectedStats.StatusNotFinal))
				p.stats.statusNotSeen.Set(float64(collectedStats.StatusNotSeen))
				p.stats.statusSeenOnNetworkTotal.Set(float64(collectedStats.StatusSeenOnNetworkTotal))
				p.stats.statusMinedTotal.Set(float64(collectedStats.StatusMinedTotal))
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
