package handler

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

var ErrFailedToRegisterStats = errors.New("failed to register stats collector")

type Stats struct {
	apiTxSubmissions prometheus.Counter
}

func NewStats() (*Stats, error) {
	p := &Stats{
		apiTxSubmissions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "api_submit_txs",
			Help: "Nr of txs submitted",
		}),
	}

	err := prometheus.Register(p.apiTxSubmissions)
	if err != nil {
		return nil, errors.Join(ErrFailedToRegisterStats, err)
	}

	return p, nil
}

func (s *Stats) Add(inc int) {
	s.apiTxSubmissions.Add(float64(inc))
}

func (s *Stats) UnregisterStats() {
	_ = prometheus.Unregister(s.apiTxSubmissions)
}
