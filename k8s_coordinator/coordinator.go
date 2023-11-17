package k8s_coordinator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
)

type K8sClient interface {
	GetPodNames(ctx context.Context, namespace string) ([]string, error)
}

type Coordinator struct {
	Client           metamorph_api.MetaMorphAPIClient
	K8sClient        K8sClient
	logger           *slog.Logger
	namespace        string
	shutdown         chan struct{}
	shutdownComplete chan struct{}
}

func WithLogger(logger *slog.Logger) func(*Coordinator) {
	return func(p *Coordinator) {
		p.logger = logger.With(slog.String("service", "k8s-coordinator"))
	}
}

type ServerOption func(f *Coordinator)

func New(client metamorph_api.MetaMorphAPIClient, k8sClient K8sClient, namespace string, opts ...ServerOption) *Coordinator {

	coordinator := &Coordinator{
		Client:    client,
		K8sClient: k8sClient,
		namespace: namespace,
	}
	for _, opt := range opts {
		opt(coordinator)
	}

	return coordinator
}

func (c *Coordinator) Start() error {

	var activePodsStorage []string

	go func() {
		defer func() {
			c.shutdownComplete <- struct{}{}
		}()

		for {
			select {
			case <-time.NewTicker(15 * time.Second).C:
				ctx := context.Background()
				activePodsK8s, err := c.K8sClient.GetPodNames(ctx, c.namespace)
				if err != nil {
					c.logger.Error("failed to get pods", slog.String("err", err.Error()))
					continue
				}

				activePodsMap := map[string]struct{}{}

				for _, podName := range activePodsK8s {
					activePodsMap[podName] = struct{}{}
				}

				for _, podName := range activePodsStorage {
					_, found := activePodsMap[podName]
					if !found {
						resp, err := c.Client.SetUnlockedByName(ctx, &metamorph_api.SetUnlockedByNameRequest{Name: podName})
						if err != nil {
							c.logger.Error("failed to unlock metamorph records", slog.String("pod-name", podName))
							continue
						}

						c.logger.Info(fmt.Sprintf("unlocked %d records", resp.RecordsAffected))
					}
				}

				activePodsStorage = activePodsK8s

			case <-c.shutdown:
				return
			}
		}
	}()

	return nil
}

func (c *Coordinator) Shutdown() {
	c.shutdown <- struct{}{}
	<-c.shutdownComplete
}
