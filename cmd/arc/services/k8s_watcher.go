package services

import (
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher/k8s_client"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

func StartK8sWatcher(logger *slog.Logger, k8sWatcherCfg *config.K8sWatcherConfig, commonCfg *config.CommonConfig) (func(), error) {
	logger.With(slog.String("service", "k8s-watcher"))

	mtmConn, err := grpc_utils.DialGRPC(k8sWatcherCfg.MetamorphDialAddr, commonCfg.Prometheus.Endpoint, commonCfg.GrpcMessageSize, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	metamorphClient := metamorph_api.NewMetaMorphAPIClient(mtmConn)

	k8sClient, err := k8s_client.New()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s-client: %v", err)
	}

	k8sWatcher := k8s_watcher.New(logger, metamorphClient, k8sClient, k8sWatcherCfg.Namespace)
	err = k8sWatcher.Start()
	if err != nil {
		return nil, fmt.Errorf("faile to start k8s-watcher: %v", err)
	}

	return func() {
		logger.Info("Shutting down K8s watcher")
		k8sWatcher.Shutdown()
	}, nil
}
