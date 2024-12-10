package cmd

import (
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher/k8s_client"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
)

func StartK8sWatcher(logger *slog.Logger, arcConfig *config.ArcConfig) (func(), error) {
	logger.With(slog.String("service", "k8s-watcher"))

	mtmConn, err := metamorph.DialGRPC(arcConfig.Metamorph.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	metamorphClient := metamorph.NewClient(metamorph_api.NewMetaMorphAPIClient(mtmConn))

	k8sClient, err := k8s_client.New()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s-client: %v", err)
	}

	blocktxConn, err := blocktx.DialGRPC(arcConfig.Blocktx.DialAddr, arcConfig.Prometheus.Endpoint, arcConfig.GrpcMessageSize, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	blocktxClient := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(blocktxConn))

	k8sWatcher := k8s_watcher.New(metamorphClient, blocktxClient, k8sClient, arcConfig.K8sWatcher.Namespace, k8s_watcher.WithLogger(logger))
	err = k8sWatcher.Start()
	if err != nil {
		return nil, fmt.Errorf("faile to start k8s-watcher: %v", err)
	}

	return func() {
		logger.Info("Shutting down K8s watcher")
		k8sWatcher.Shutdown()
	}, nil
}
