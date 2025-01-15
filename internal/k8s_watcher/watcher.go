package k8s_watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph"
)

const (
	logLevelDefault      = slog.LevelInfo
	metamorphService     = "metamorph"
	intervalDefault      = 15 * time.Second
	maxRetries           = 5
	retryIntervalDefault = 2 * time.Second
)

type K8sClient interface {
	GetRunningPodNames(ctx context.Context, namespace string, service string) (map[string]struct{}, error)
}

type Watcher struct {
	metamorphClient   metamorph.TransactionMaintainer
	k8sClient         K8sClient
	logger            *slog.Logger
	tickerMetamorph   Ticker
	tickerBlocktx     Ticker
	namespace         string
	waitGroup         *sync.WaitGroup
	shutdownMetamorph context.CancelFunc
	shutdownBlocktx   context.CancelFunc
	retryInterval     time.Duration
}

func WithLogger(logger *slog.Logger) func(*Watcher) {
	return func(p *Watcher) {
		p.logger = logger.With(slog.String("service", "k8s-watcher"))
	}
}

func WithRetryInterval(d time.Duration) func(*Watcher) {
	return func(p *Watcher) {
		p.retryInterval = d
	}
}

type ServerOption func(f *Watcher)

// New The K8s watcher listens to events coming from Kubernetes. If it detects a metamorph pod which was terminated, then it sets records locked by this pod to unlocked. This is a safety measure for the case that metamorph is terminated ungracefully where it misses to unlock its records itself.
func New(metamorphClient metamorph.TransactionMaintainer, k8sClient K8sClient, namespace string, opts ...ServerOption) *Watcher {
	watcher := &Watcher{
		metamorphClient: metamorphClient,
		k8sClient:       k8sClient,

		namespace:       namespace,
		logger:          slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "k8s-watcher")),
		tickerMetamorph: NewDefaultTicker(intervalDefault),
		tickerBlocktx:   NewDefaultTicker(intervalDefault),
		waitGroup:       &sync.WaitGroup{},
		retryInterval:   retryIntervalDefault,
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

func WithMetamorphTicker(t Ticker) func(*Watcher) {
	return func(p *Watcher) {
		p.tickerMetamorph = t
	}
}

func (c *Watcher) Start() error {
	c.watchMetamorph()

	return nil
}

func (c *Watcher) watchMetamorph() {
	ctx, cancel := context.WithCancel(context.Background())
	c.shutdownMetamorph = cancel
	c.waitGroup.Add(1)
	go func() {
		defer c.waitGroup.Done()
		var runningPods map[string]struct{}

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.tickerMetamorph.Tick():
				// Update the list of running pods. Detect those which have been terminated and unlock records for these pods
				ctx := context.Background()
				runningPodsK8s, err := c.k8sClient.GetRunningPodNames(ctx, c.namespace, metamorphService)
				if err != nil {
					c.logger.Error("failed to get pods", slog.String("err", err.Error()))
					continue
				}

				for podName := range runningPods {
					// Ignore all other services than metamorph
					if !strings.Contains(podName, metamorphService) {
						continue
					}

					_, found := runningPodsK8s[podName]
					if !found {
						// A previously running pod has been terminated => set records locked by this pod unlocked

						retryTicker := time.NewTicker(c.retryInterval)
						i := 0

					retryLoop:
						for range retryTicker.C {
							i++

							if i > maxRetries {
								c.logger.Error(fmt.Sprintf("Failed to unlock metamorph records after %d retries", maxRetries), slog.String("pod-name", podName))
								break retryLoop
							}

							rows, err := c.metamorphClient.SetUnlockedByName(ctx, podName)
							if err != nil {
								c.logger.Error("Failed to unlock metamorph records", slog.String("pod-name", podName), slog.String("err", err.Error()))
								continue
							}
							c.logger.Info("Records unlocked", slog.Int64("rows-affected", rows), slog.String("pod-name", podName))
							break
						}
					}
				}

				runningPods = runningPodsK8s
			}
		}
	}()
}

func (c *Watcher) Shutdown() {
	if c.shutdownMetamorph != nil {
		c.shutdownMetamorph()
	}

	if c.shutdownBlocktx != nil {
		c.shutdownBlocktx()
	}

	c.waitGroup.Wait()
}
