package cmd

import (
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher/k8s_client"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

func StartK8sWatcher(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	logger.With(slog.String("service", "k8s-watcher"))

	callbackerConn, err := metamorph.DialGRPC(arcConfig.Callbacker.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create callbacker client: %v", err)
	}

	mtmConn, err := metamorph.DialGRPC(arcConfig.Metamorph.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	metamorphClient := metamorph.NewClient(metamorph_api.NewMetaMorphAPIClient(mtmConn))
	callbackerClient := callbacker_api.NewCallbackerAPIClient(callbackerConn)

	k8sClient, err := k8s_client.New()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s-client: %v", err)
	}

	k8sWatcher := k8s_watcher.New(metamorphClient, callbackerClient, k8sClient, arcConfig.K8sWatcher.Namespace, k8s_watcher.WithLogger(logger))
	err = k8sWatcher.Start()
	if err != nil {
		return nil, fmt.Errorf("faile to start k8s-watcher: %v", err)
	}

	return func() {
		logger.Info("Shutting down K8s watcher")
		k8sWatcher.Shutdown()
	}, nil
}
