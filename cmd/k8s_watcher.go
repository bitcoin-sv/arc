package cmd

import (
	"fmt"
	"github.com/bitcoin-sv/arc/config"
	"log/slog"

	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/k8s_watcher"
	"github.com/bitcoin-sv/arc/k8s_watcher/k8s_client"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
)

func StartK8sWatcher(logger *slog.Logger) (func(), error) {
	logger.With(slog.String("service", "k8s-watcher"))

	metamorphAddress, err := config.GetString("metamorph.dialAddr")
	if err != nil {
		return nil, err
	}
	grpcMessageSize, err := config.GetInt("grpcMessageSize")
	if err != nil {
		return nil, err
	}

	metamorphConn, err := transactionHandler.DialGRPC(metamorphAddress, grpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection to metamorph with address %s: %v", metamorphAddress, err)
	}

	metamorphClient := metamorph_api.NewMetaMorphAPIClient(metamorphConn)

	k8sClient, err := k8s_client.New()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s-client: %v", err)
	}

	namespace, err := config.GetString("k8sWatcher.namespace")
	if err != nil {
		return nil, err
	}

	k8sWatcher := k8s_watcher.New(metamorphClient, k8sClient, namespace, k8s_watcher.WithLogger(logger))
	err = k8sWatcher.Start()
	if err != nil {
		return nil, fmt.Errorf("faile to start k8s-watcher: %v", err)
	}

	return func() {
		logger.Info("Shutting down K8s watcher")
		k8sWatcher.Shutdown()
	}, nil
}
