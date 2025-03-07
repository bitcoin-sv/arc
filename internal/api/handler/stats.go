package handler

import (
	"errors"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var ErrFailedToRegisterStats = errors.New("failed to register stats collector")

type Stats struct {
	mu               sync.RWMutex
	apiTxSubmissions prometheus.Counter
}

func NewStats() *Stats {
	p := &Stats{
		apiTxSubmissions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "api_submit_txs",
			Help: "Nr of txs submitted",
		}),
	}

	return p
}

func (s *Stats) Add(inc int) {
	s.mu.Lock()
	s.apiTxSubmissions.Add(float64(inc))
	s.mu.Unlock()
}

func (s *Stats) RegisterStats() error {
	err := prometheus.Register(s.apiTxSubmissions)
	if err != nil {
		return errors.Join(ErrFailedToRegisterStats, err)
	}

	return nil
}

func (s *Stats) UnregisterStats() {
	_ = prometheus.Unregister(s.apiTxSubmissions)
}
