package metamorph

import (
	"github.com/ordishs/go-utils/safemap"
	"github.com/prometheus/client_golang/prometheus"
)

type zmqCollector struct {
	zmqStats             *safemap.Safemap[string, *ZMQStats]
	hashTx               *prometheus.Desc
	invalidTx            *prometheus.Desc
	discardedFromMempool *prometheus.Desc
}

// NewZMQCollector initializes every descriptor and returns a pointer to the prometheusCollector
func NewZMQCollector(zmqStats *safemap.Safemap[string, *ZMQStats]) *zmqCollector {
	c := &zmqCollector{
		zmqStats: zmqStats,
		hashTx: prometheus.NewDesc("arc_metamorph_zmq_hashtx",
			"Shows the number of hashTx messages received",
			[]string{"peer"}, nil,
		),
		invalidTx: prometheus.NewDesc("arc_metamorph_zmq_invalidtx",
			"Shows the number of invalidTx messages received",
			[]string{"peer"}, nil,
		),
		discardedFromMempool: prometheus.NewDesc("arc_metamorph_zmq_discardedfrommempool",
			"Shows the number of discardedFromMempool messages received",
			[]string{"peer"}, nil,
		),
	}

	prometheus.MustRegister(c)

	return c
}

// Describe writes all descriptors to the prometheus desc channel.
func (c *zmqCollector) Describe(ch chan<- *prometheus.Desc) {

	ch <- c.hashTx
	ch <- c.invalidTx
	ch <- c.discardedFromMempool
}

// Collect implements required collect function for all prometheus collectors
func (c *zmqCollector) Collect(ch chan<- prometheus.Metric) {

	//Note that you can pass CounterValue, GaugeValue, or UntypedValue types here.
	c.zmqStats.Each(func(peer string, zmqStats *ZMQStats) {
		ch <- prometheus.MustNewConstMetric(c.hashTx, prometheus.CounterValue, float64(zmqStats.hashTx.Load()), peer)
		ch <- prometheus.MustNewConstMetric(c.invalidTx, prometheus.CounterValue, float64(zmqStats.invalidTx.Load()), peer)
		ch <- prometheus.MustNewConstMetric(c.discardedFromMempool, prometheus.CounterValue, float64(zmqStats.discardedFromMempool.Load()), peer)
	})
}
