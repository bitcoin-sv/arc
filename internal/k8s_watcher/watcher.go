package k8s_watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

const (
	metamorphService  = "metamorph"
	callbackerService = "callbacker"
	intervalDefault   = 15 * time.Second
)

type K8sClient interface {
	GetRunningPodNames(ctx context.Context, namespace string, service string) (map[string]struct{}, error)
	GetRunningPodNamesSlice(ctx context.Context, namespace string, podName string) ([]string, error)
}

type Watcher struct {
	metamorphClient  metamorph_api.MetaMorphAPIClient
	callbackerClient callbacker_api.CallbackerAPIClient
	k8sClient        K8sClient
	logger           *slog.Logger
	updateInterval   time.Duration
	namespace        string
	waitGroup        *sync.WaitGroup
	cancellations    []context.CancelFunc
}

type ServerOption func(f *Watcher)

// New The K8s watcher listens to events coming from Kubernetes. If it detects a metamorph pod which was terminated, then it sets records locked by this pod to unlocked. This is a safety measure for the case that metamorph is terminated ungracefully where it misses to unlock its records itself.
func New(logger *slog.Logger, metamorphClient metamorph_api.MetaMorphAPIClient, callbackerClient callbacker_api.CallbackerAPIClient, k8sClient K8sClient, namespace string, opts ...ServerOption) *Watcher {
	watcher := &Watcher{
		metamorphClient:  metamorphClient,
		callbackerClient: callbackerClient,
		k8sClient:        k8sClient,

		namespace:      namespace,
		logger:         logger,
		updateInterval: intervalDefault,
		waitGroup:      &sync.WaitGroup{},
	}
	for _, opt := range opts {
		opt(watcher)
	}

	return watcher
}

func (c *Watcher) Start() error {
	c.updateRunningPods(metamorphService, c.updateInterval, func(ctx context.Context, podNames []string) error {
		response, err := c.metamorphClient.UpdateInstances(ctx, &metamorph_api.UpdateInstancesRequest{Instances: podNames})
		if err == nil && response.Response != "" {
			c.logger.Info("Updated instances", slog.String("service", metamorphService), slog.String("response", response.Response))
		}

		return err
	})

	c.updateRunningPods(callbackerService, c.updateInterval, func(ctx context.Context, podNames []string) error {
		response, err := c.callbackerClient.UpdateInstances(ctx, &callbacker_api.UpdateInstancesRequest{Instances: podNames})
		if err == nil && response.Response != "" {
			c.logger.Info("Updated instances", slog.String("service", callbackerService), slog.String("response", response.Response))
		}

		return err
	})

	return nil
}

// updateRunningPods Runs a specified update function periodically with a list of running pods of a given service
func (c *Watcher) updateRunningPods(service string, updateInterval time.Duration, updateFunc func(context.Context, []string) error) {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancellations = append(c.cancellations, cancel)

	ticker := time.NewTicker(updateInterval)

	c.waitGroup.Add(1)
	go func() {
		defer c.waitGroup.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Update the list of running pods. Detect those which have been terminated and unlock records for these pods
				runningPodsK8s, err := c.k8sClient.GetRunningPodNamesSlice(ctx, c.namespace, service)
				if err != nil {
					c.logger.Error("failed to get pods", slog.String("err", err.Error()))
					continue
				}

				err = updateFunc(ctx, runningPodsK8s)
				if err != nil {
					c.logger.Error("Failed to run update function", slog.String("err", err.Error()))
					continue
				}
			}
		}
	}()
}

func (c *Watcher) Shutdown() {
	for _, cancel := range c.cancellations {
		cancel()
	}

	c.waitGroup.Wait()
}
