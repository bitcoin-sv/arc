package k8s_coordinator

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
)

const (
	logLevelDefault  = slog.LevelInfo
	intervalDefault  = 15 * time.Second
	metamorphService = "metamorph"
)

type K8sClient interface {
	GetPodNames(ctx context.Context, namespace string) (map[string]struct{}, error)
}

type Ticker interface {
	Stop()
	Tick() <-chan time.Time
}

type DefaultTicker struct {
	*time.Ticker
}

func NewDefaultTicker(d time.Duration) DefaultTicker {
	return DefaultTicker{time.NewTicker(d)}
}

func (d DefaultTicker) Tick() <-chan time.Time {
	return d.C
}

type Coordinator struct {
	metamorphClient  metamorph_api.MetaMorphAPIClient
	k8sClient        K8sClient
	logger           *slog.Logger
	ticker           Ticker
	namespace        string
	shutdown         chan struct{}
	shutdownComplete chan struct{}
}

func WithLogger(logger *slog.Logger) func(*Coordinator) {
	return func(p *Coordinator) {
		p.logger = logger.With(slog.String("service", "k8s-coordinator"))
	}
}
func WithTicker(t Ticker) func(*Coordinator) {
	return func(p *Coordinator) {
		p.ticker = t
	}
}

type ServerOption func(f *Coordinator)

// New The K8s coordinator periodically checks which metamorph pods are running. If it detects a metamorph pod which was terminated, then it sets records locked by this pod to unlocked. This is a safety measure for the case that metamorph is terminated ungracefully where it misses to unlock its records itself.
func New(client metamorph_api.MetaMorphAPIClient, k8sClient K8sClient, namespace string, opts ...ServerOption) *Coordinator {
	coordinator := &Coordinator{
		metamorphClient: client,
		k8sClient:       k8sClient,

		namespace:        namespace,
		logger:           slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "k8s-coordinator")),
		shutdown:         make(chan struct{}),
		shutdownComplete: make(chan struct{}),
		ticker:           NewDefaultTicker(intervalDefault),
	}
	for _, opt := range opts {
		opt(coordinator)
	}

	return coordinator
}

func (c *Coordinator) Start() error {
	go func() {
		defer func() {
			c.shutdownComplete <- struct{}{}
		}()

		var activePods map[string]struct{}

		for {
			select {
			case <-c.ticker.Tick():
				// Update the list of running pods. Detect those which have been terminated and unlock records for these pods
				ctx := context.Background()
				activePodsK8s, err := c.k8sClient.GetPodNames(ctx, c.namespace)
				if err != nil {
					c.logger.Error("failed to get pods", slog.String("err", err.Error()))
					continue
				}

				for podName := range activePods {
					// Ignore all other serivces than metamorph
					if !strings.Contains(podName, metamorphService) {
						continue
					}

					_, found := activePodsK8s[podName]
					if !found {
						// A previously running pod has been terminated => set records locked by this pod unlocked
						resp, err := c.metamorphClient.SetUnlockedByName(ctx, &metamorph_api.SetUnlockedByNameRequest{Name: podName})
						if err != nil {
							c.logger.Error("failed to unlock metamorph records", slog.String("pod-name", podName))
							continue
						}

						c.logger.Info("records unlocked", slog.Int("rows-affected", int(resp.RecordsAffected)), slog.String("pod-name", podName))
					}
				}

				activePods = activePodsK8s

			case <-c.shutdown:
				return
			}
		}
	}()

	return nil
}

func (c *Coordinator) Shutdown() {
	c.ticker.Stop()
	c.shutdown <- struct{}{}
	<-c.shutdownComplete
}
