package k8s_coordinator

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
)

const (
	logLevelDefault = slog.LevelInfo
	intervalDefault = 15 * time.Second
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

		var activePods []string

		for {
			select {
			case <-c.ticker.Tick():
				ctx := context.Background()
				activePodsK8s, err := c.k8sClient.GetPodNames(ctx, c.namespace)
				if err != nil {
					c.logger.Error("failed to get pods", slog.String("err", err.Error()))
					continue
				}

				for _, podName := range activePods {
					_, found := activePodsK8s[podName]
					if !found {
						// one of the previously running pods is not available anymore => set records locked by this pod unlocked
						resp, err := c.metamorphClient.SetUnlockedByName(ctx, &metamorph_api.SetUnlockedByNameRequest{Name: podName})
						if err != nil {
							c.logger.Error("failed to unlock metamorph records", slog.String("pod-name", podName))
							continue
						}

						c.logger.Info("records unlocked", slog.Int("rows-affected", int(resp.RecordsAffected)), slog.String("pod-name", podName))
					}
				}

				newActivePods := make([]string, len(activePodsK8s))
				index := 0
				for podName := range activePodsK8s {
					newActivePods[index] = podName
					index++
				}

				activePods = newActivePods

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
