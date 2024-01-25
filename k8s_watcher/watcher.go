package k8s_watcher

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
)

const (
	logLevelDefault  = slog.LevelInfo
	metamorphService = "metamorph"
	intervalDefault  = 15 * time.Second
)

type K8sClient interface {
	GetRunningPodNames(ctx context.Context, namespace string, service string) (map[string]struct{}, error)
}

type Watcher struct {
	metamorphClient  metamorph.TransactionMaintainer
	k8sClient        K8sClient
	logger           *slog.Logger
	ticker           Ticker
	namespace        string
	shutdown         chan struct{}
	shutdownComplete chan struct{}
}

func WithLogger(logger *slog.Logger) func(*Watcher) {
	return func(p *Watcher) {
		p.logger = logger.With(slog.String("service", "k8s-watcher"))
	}
}

type ServerOption func(f *Watcher)

// New The K8s watcher listens to events coming from Kubernetes. If it detects a metamorph pod which was terminated, then it sets records locked by this pod to unlocked. This is a safety measure for the case that metamorph is terminated ungracefully where it misses to unlock its records itself.
func New(client metamorph.TransactionMaintainer, k8sClient K8sClient, namespace string, opts ...ServerOption) *Watcher {
	watcher := &Watcher{
		metamorphClient: client,
		k8sClient:       k8sClient,

		namespace:        namespace,
		logger:           slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "k8s-watcher")),
		shutdown:         make(chan struct{}),
		shutdownComplete: make(chan struct{}),
		ticker:           NewDefaultTicker(intervalDefault),
	}
	for _, opt := range opts {
		opt(watcher)
	}

	return watcher
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

func WithTicker(t Ticker) func(*Watcher) {
	return func(p *Watcher) {
		p.ticker = t
	}
}

func (c *Watcher) Start() error {
	go func() {
		defer func() {
			c.shutdownComplete <- struct{}{}
		}()

		var runningPods map[string]struct{}

		for {
			select {
			case <-c.ticker.Tick():
				// Update the list of running pods. Detect those which have been terminated and unlock records for these pods
				ctx := context.Background()
				runningPodsK8s, err := c.k8sClient.GetRunningPodNames(ctx, c.namespace, metamorphService)
				if err != nil {
					c.logger.Error("failed to get pods", slog.String("err", err.Error()))
					continue
				}

				for podName := range runningPods {
					// Ignore all other serivces than metamorph
					if !strings.Contains(podName, metamorphService) {
						continue
					}

					_, found := runningPodsK8s[podName]
					if !found {
						// A previously running pod has been terminated => set records locked by this pod unlocked
						resp, err := c.metamorphClient.SetUnlockedByName(ctx, &metamorph_api.SetUnlockedByNameRequest{Name: podName})
						if err != nil {
							c.logger.Error("failed to unlock metamorph records", slog.String("pod-name", podName), slog.String("err", err.Error()))
							continue
						}

						c.logger.Info("records unlocked", slog.Int64("rows-affected", resp.RecordsAffected), slog.String("pod-name", podName))
					}
				}

				runningPods = runningPodsK8s

			case <-c.shutdown:
				return
			}
		}
	}()

	return nil
}

func (c *Watcher) Shutdown() {
	c.ticker.Stop()
	c.shutdown <- struct{}{}
	<-c.shutdownComplete
}
