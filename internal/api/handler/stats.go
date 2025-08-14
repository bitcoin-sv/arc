package handler

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

var ErrFailedToRegisterStats = errors.New("failed to register stats collector")

type Stats struct {
	apiTxSubmissions               prometheus.Counter
	AvailableBlockHeaderServices   prometheus.Gauge
	UnavailableBlockHeaderServices prometheus.Gauge
}

func NewStats() (*Stats, error) {
	p := &Stats{
		apiTxSubmissions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "api_submit_txs",
			Help: "Nr of txs submitted",
		}),
		AvailableBlockHeaderServices: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_api_available_block_header_services",
			Help: "Current number of available block header services",
		}),
		UnavailableBlockHeaderServices: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "arc_api_unavailable_block_header_services",
			Help: "Current number of unavailable block header services",
		}),
	}

	err := registerStats(
		p.apiTxSubmissions,
		p.AvailableBlockHeaderServices,
		p.UnavailableBlockHeaderServices,
	)
	if err != nil {
		return nil, errors.Join(ErrFailedToRegisterStats, err)
	}

	return p, nil
}

func (s *Stats) Add(inc int) {
	s.apiTxSubmissions.Add(float64(inc))
}

func (s *Stats) UnregisterStats() {
	unregisterStats(
		s.apiTxSubmissions,
		s.AvailableBlockHeaderServices,
		s.UnavailableBlockHeaderServices,
	)
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
