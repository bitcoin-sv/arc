package metamorph

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

type prometheusCollector struct {
	processor                 ProcessorI
	worker                    *prometheus.Desc
	queueLength               *prometheus.Desc
	queued                    *prometheus.Desc
	stored                    *prometheus.Desc
	storedDuration            *prometheus.Desc
	announced                 *prometheus.Desc
	announcedDuration         *prometheus.Desc
	sentToNetwork             *prometheus.Desc
	sentToNetworkDuration     *prometheus.Desc
	acceptedByNetwork         *prometheus.Desc
	acceptedByNetworkDuration *prometheus.Desc
	seenOnNetwork             *prometheus.Desc
	seenOnNetworkDuration     *prometheus.Desc
	rejected                  *prometheus.Desc
	rejectedDuration          *prometheus.Desc
	mined                     *prometheus.Desc
	minedDuration             *prometheus.Desc
	retries                   *prometheus.Desc
	retriesDuration           *prometheus.Desc
	channelMapSize            *prometheus.Desc
}

var collectorLoaded = atomic.Bool{}

// You must create a constructor for you prometheusCollector that
// initializes every descriptor and returns a pointer to the prometheusCollector
func newPrometheusCollector(p ProcessorI) *prometheusCollector {
	if !collectorLoaded.CompareAndSwap(false, true) {
		return nil
	}

	c := &prometheusCollector{
		processor: p,
		worker: prometheus.NewDesc("arc_metamorph_processor_worker",
			"Shows the number of workers in the processor",
			nil, nil,
		),
		queueLength: prometheus.NewDesc("arc_metamorph_processor_queue_length",
			"Shows the number of ResponseItems in the processor queue",
			nil, nil,
		),
		queued: prometheus.NewDesc("arc_metamorph_processor_queued",
			"Shows the number of ResponseItems queued in the processor",
			nil, nil,
		),
		stored: prometheus.NewDesc("arc_metamorph_processor_stored",
			"Shows the number of ResponseItems stored by the processor",
			nil, nil,
		),
		storedDuration: prometheus.NewDesc("arc_metamorph_processor_stored_duration_ms",
			"Shows the duration it took to store all ResponseItems by the processor",
			nil, nil,
		),
		announced: prometheus.NewDesc("arc_metamorph_processor_announced",
			"Shows the number of ResponseItems announced by the processor",
			nil, nil,
		),
		announcedDuration: prometheus.NewDesc("arc_metamorph_processor_announced_duration_ms",
			"Shows the duration it took to to announce all ResponseItems by the processor",
			nil, nil,
		),
		sentToNetwork: prometheus.NewDesc("arc_metamorph_processor_sent_to_network",
			"Shows the number of ResponseItems sent to the network by the processor",
			nil, nil,
		),
		sentToNetworkDuration: prometheus.NewDesc("arc_metamorph_processor_sent_to_network_duration_ms",
			"Shows the duration it took to send all ResponseItems to the network by the processor",
			nil, nil,
		),
		acceptedByNetwork: prometheus.NewDesc("arc_metamorph_processor_accepted_by_network",
			"Shows the number of ResponseItems accepted by the network by the processor",
			nil, nil,
		),
		acceptedByNetworkDuration: prometheus.NewDesc("arc_metamorph_processor_accepted_by_network_duration_ms",
			"Shows the duration it took to accept all ResponseItems by the network by the processor",
			nil, nil,
		),
		seenOnNetwork: prometheus.NewDesc("arc_metamorph_processor_seen_on_network",
			"Shows the number of ResponseItems seen on the network by the processor",
			nil, nil,
		),
		seenOnNetworkDuration: prometheus.NewDesc("arc_metamorph_processor_seen_on_network_duration_ms",
			"Shows the duration it took to see all ResponseItems on the network by the processor",
			nil, nil,
		),
		rejected: prometheus.NewDesc("arc_metamorph_processor_rejected",
			"Shows the number of ResponseItems rejected by the processor",
			nil, nil,
		),
		rejectedDuration: prometheus.NewDesc("arc_metamorph_processor_rejected_duration_ms",
			"Shows the duration it took to reject all ResponseItems by the processor",
			nil, nil,
		),
		mined: prometheus.NewDesc("arc_metamorph_processor_mined",
			"Shows the number of ResponseItems mined by the processor",
			nil, nil,
		),
		minedDuration: prometheus.NewDesc("arc_metamorph_processor_mined_duration_ms",
			"Shows the duration it took to mine all ResponseItems by the processor",
			nil, nil,
		),
		retries: prometheus.NewDesc("arc_metamorph_processor_retries",
			"Shows the number of ResponseItems retried by the processor",
			nil, nil,
		),
		retriesDuration: prometheus.NewDesc("arc_metamorph_processor_retries_duration_ms",
			"Shows the duration it took to retry all ResponseItems by the processor",
			nil, nil,
		),
		channelMapSize: prometheus.NewDesc("arc_metamorph_processor_map_size",
			"Shows the number of ResponseItems in the processor map",
			nil, nil,
		),
	}

	prometheus.MustRegister(c)

	return c
}

// Describe writes all descriptors to the prometheus desc channel.
func (c *prometheusCollector) Describe(ch chan<- *prometheus.Desc) {

	//Update this section with each metric you create for a given prometheusCollector
	ch <- c.worker
	ch <- c.queueLength
	ch <- c.queued
	ch <- c.stored
	ch <- c.storedDuration
	ch <- c.announced
	ch <- c.announcedDuration
	ch <- c.sentToNetwork
	ch <- c.sentToNetworkDuration
	ch <- c.acceptedByNetwork
	ch <- c.acceptedByNetworkDuration
	ch <- c.seenOnNetwork
	ch <- c.seenOnNetworkDuration
	ch <- c.rejected
	ch <- c.rejectedDuration
	ch <- c.mined
	ch <- c.minedDuration
	ch <- c.retries
	ch <- c.retriesDuration
	ch <- c.channelMapSize
}

// Collect implements required collect function for all prometheus collectors
func (c *prometheusCollector) Collect(ch chan<- prometheus.Metric) {

	stats := c.processor.GetStats(false)

	//Write the latest value for each metric in the prometheus metric channel.
	//Note that you can pass erValue, GaugeValue, or UntypedValue types here.
	ch <- prometheus.MustNewConstMetric(c.queueLength, prometheus.GaugeValue, float64(stats.QueueLength))
	ch <- prometheus.MustNewConstMetric(c.queued, prometheus.CounterValue, float64(stats.QueuedCount))
	ch <- prometheus.MustNewConstMetric(c.stored, prometheus.CounterValue, float64(stats.Stored.GetCount()))
	ch <- prometheus.MustNewConstMetric(c.storedDuration, prometheus.CounterValue, float64(stats.Stored.GetDuration().Milliseconds()))
	ch <- prometheus.MustNewConstMetric(c.announced, prometheus.CounterValue, float64(stats.AnnouncedToNetwork.GetCount()))
	ch <- prometheus.MustNewConstMetric(c.announcedDuration, prometheus.CounterValue, float64(stats.AnnouncedToNetwork.GetTotalDuration().Milliseconds()))
	ch <- prometheus.MustNewConstMetric(c.sentToNetwork, prometheus.CounterValue, float64(stats.SentToNetwork.GetCount()))
	ch <- prometheus.MustNewConstMetric(c.sentToNetworkDuration, prometheus.CounterValue, float64(stats.SentToNetwork.GetTotalDuration().Milliseconds()))
	ch <- prometheus.MustNewConstMetric(c.acceptedByNetwork, prometheus.CounterValue, float64(stats.AcceptedByNetwork.GetCount()))
	ch <- prometheus.MustNewConstMetric(c.acceptedByNetworkDuration, prometheus.CounterValue, float64(stats.AcceptedByNetwork.GetTotalDuration().Milliseconds()))
	ch <- prometheus.MustNewConstMetric(c.seenOnNetwork, prometheus.CounterValue, float64(stats.SeenOnNetwork.GetCount()))
	ch <- prometheus.MustNewConstMetric(c.seenOnNetworkDuration, prometheus.CounterValue, float64(stats.SeenOnNetwork.GetTotalDuration().Milliseconds()))
	ch <- prometheus.MustNewConstMetric(c.rejected, prometheus.CounterValue, float64(stats.Rejected.GetCount()))
	ch <- prometheus.MustNewConstMetric(c.rejectedDuration, prometheus.CounterValue, float64(stats.Rejected.GetTotalDuration().Milliseconds()))
	ch <- prometheus.MustNewConstMetric(c.mined, prometheus.CounterValue, float64(stats.Mined.GetCount()))
	ch <- prometheus.MustNewConstMetric(c.minedDuration, prometheus.CounterValue, stats.Mined.GetAverage())
	ch <- prometheus.MustNewConstMetric(c.retries, prometheus.CounterValue, float64(stats.Retries.GetCount()))
	ch <- prometheus.MustNewConstMetric(c.retriesDuration, prometheus.CounterValue, stats.Retries.GetAverage())
	ch <- prometheus.MustNewConstMetric(c.channelMapSize, prometheus.CounterValue, float64(stats.ChannelMapSize))
}
