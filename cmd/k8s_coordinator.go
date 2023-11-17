package cmd

import (
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/k8s_coordinator"
	"github.com/bitcoin-sv/arc/k8s_coordinator/k8s_client"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/spf13/viper"
)

func StartK8sCoordinator(logger *slog.Logger) (func(), error) {

	metamorphAddress := viper.GetString("metamorph.dialAddr")
	if metamorphAddress == "" {
		return nil, fmt.Errorf("metamorph.dialAddr not found in config")
	}

	grpcMessageSize := viper.GetInt("grpcMessageSize")
	if grpcMessageSize == 0 {
		return nil, fmt.Errorf("grpcMessageSize not found in config")
	}

	metamorphConn, err := transactionHandler.GetConnection(metamorphAddress, grpcMessageSize)
	if err != nil {
		return nil, err
	}

	metamorphClient := metamorph_api.NewMetaMorphAPIClient(metamorphConn)

	k8sClient, err := k8s_client.New()
	if err != nil {
		return nil, err
	}

	namespace := viper.GetString("k8sCoordinator.namespace")
	if namespace == "" {
		return nil, fmt.Errorf("k8sCoordinator.namespace not found in config")
	}

	k8sCoordinator := k8s_coordinator.New(metamorphClient, k8sClient, namespace, k8s_coordinator.WithLogger(logger))
	if err != nil {
		return nil, err
	}

	err = k8sCoordinator.Start()
	if err != nil {
		return nil, err
	}

	return func() {
		logger.Info("Shutting down K8s coordinator")
		k8sCoordinator.Shutdown()
	}, nil
}
