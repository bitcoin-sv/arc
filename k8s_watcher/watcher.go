package k8s_watcher

import (
	"context"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/davecgh/go-spew/spew"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"log/slog"
	"os"
)

const (
	logLevelDefault  = slog.LevelInfo
	metamorphService = "metamorph"
)

type K8sClient interface {
	GetPodWatcher(ctx context.Context, namespace string, podName string) (watch.Interface, error)
}

type Watcher struct {
	metamorphClient  metamorph_api.MetaMorphAPIClient
	k8sClient        K8sClient
	logger           *slog.Logger
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
func New(client metamorph_api.MetaMorphAPIClient, k8sClient K8sClient, namespace string, opts ...ServerOption) *Watcher {
	watcher := &Watcher{
		metamorphClient: client,
		k8sClient:       k8sClient,

		namespace:        namespace,
		logger:           slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "k8s-watcher")),
		shutdown:         make(chan struct{}),
		shutdownComplete: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(watcher)
	}

	return watcher
}

func (c *Watcher) Start() error {
	ctx := context.Background()
	watcher, err := c.k8sClient.GetPodWatcher(ctx, c.namespace, metamorphService)
	if err != nil {
		c.logger.Error("failed to get pod watcher", slog.String("err", err.Error()))
		return err
	}

	go func() {
		defer func() {
			c.shutdownComplete <- struct{}{}
		}()

		for {
			select {
			case event := <-watcher.ResultChan():
				c.logger.Debug("event received", slog.String("type", string(event.Type)))

				if event.Type != watch.Deleted {
					continue
				}

				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					c.logger.Debug("event received is not a pod", slog.String("type", string(event.Type)), slog.String("object", spew.Sdump(event.Object)))
					continue
				}

				c.logger.Info("pod has been terminated", slog.String("name", pod.Name))

				resp, err := c.metamorphClient.SetUnlockedByName(ctx, &metamorph_api.SetUnlockedByNameRequest{Name: pod.Name})
				if err != nil {
					c.logger.Error("failed to unlock metamorph records", slog.String("pod-name", pod.Name))
					continue
				}

				c.logger.Info("records unlocked", slog.Int("rows-affected", int(resp.RecordsAffected)), slog.String("pod-name", pod.Name))

			case <-c.shutdown:
				watcher.Stop()
				return
			}
		}
	}()

	return nil
}

func (c *Watcher) Shutdown() {
	c.shutdown <- struct{}{}
	<-c.shutdownComplete
}
