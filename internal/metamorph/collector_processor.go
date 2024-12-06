package metamorph

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

type prometheusCollector struct {
	processor              ProcessorI
	channelMapSize         *prometheus.Desc
	healthyPeerConnections *prometheus.Desc
}

var collectorLoaded = atomic.Bool{}

// You must create a constructor for you prometheusCollector that
// initializes every descriptor and returns a pointer to the prometheusCollector
func newPrometheusCollector(p ProcessorI) error {
	if !collectorLoaded.CompareAndSwap(false, true) {
		return nil
	}

	c := &prometheusCollector{
		processor: p,
		channelMapSize: prometheus.NewDesc("arc_metamorph_processor_map_size",
			"Number of ResponseItems in the processor map",
			nil, nil,
		),
		healthyPeerConnections: prometheus.NewDesc("arc_healthy_peers_count", "Number of healthy peer connections", nil, nil),
	}

	return prometheus.Register(c)
}

// Describe writes all descriptors to the prometheus desc channel.
func (c *prometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	// Update this section with each metric you create for a given prometheusCollector
	ch <- c.channelMapSize
}

// Collect implements required collect function for all prometheus collectors
func (c *prometheusCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.processor.GetProcessorMapSize()

	// Note that you can pass erValue, GaugeValue, or UntypedValue types here.
	ch <- prometheus.MustNewConstMetric(c.channelMapSize, prometheus.GaugeValue, float64(stats))

	healthyConnections := 0

	for _, peer := range c.processor.GetPeers() {
		if peer.Connected() {
			healthyConnections++
			continue
		}
	}

	ch <- prometheus.MustNewConstMetric(c.healthyPeerConnections, prometheus.GaugeValue, float64(healthyConnections))
}
