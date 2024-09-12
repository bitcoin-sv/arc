package callbacker

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type stats struct {
	callbackSeenOnNetworkCount        prometheus.Gauge
	callbackSeenInOrphanMempoolCount  prometheus.Gauge
	callbackDoubleSpendAttemptedCount prometheus.Gauge
	callbackRejectedCount             prometheus.Gauge
	callbackMinedCount                prometheus.Gauge
	callbackFailedCount               prometheus.Gauge
}

func newCallbackerStats() *stats {
	return &stats{
		callbackSeenOnNetworkCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_callback_seen_on_network_count",
			Help: "Number of arc_callback_seen_on_network_count transactions",
		}),
		callbackSeenInOrphanMempoolCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_callback_seen_in_orphan_mempool_count",
			Help: "Number of arc_callback_seen_in_orphan_mempool_count transactions",
		}),
		callbackDoubleSpendAttemptedCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_callback_double_spend_attempted_count",
			Help: "Number of arc_callback_double_spend_attempted_count transactions",
		}),
		callbackRejectedCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_callback_rejected_count",
			Help: "Number of arc_callback_rejected_count transactions",
		}),
		callbackMinedCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_callback_mined_count",
			Help: "Number of arc_callback_mined_count transactions",
		}),
		callbackFailedCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_callback_failed_count",
			Help: "Number of arc_callback_failed_count transactions",
		}),
	}
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
