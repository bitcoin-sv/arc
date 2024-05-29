package cmd

import (
	"fmt"
	"log/slog"

	cfg "github.com/bitcoin-sv/arc/internal/config"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher/k8s_client"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/spf13/viper"
)

func StartK8sWatcher(logger *slog.Logger) (func(), error) {
	logger.With(slog.String("service", "k8s-watcher"))

	metamorphAddress, err := cfg.GetString("metamorph.dialAddr")
	if err != nil {
		return nil, err
	}

	grpcMessageSize, err := cfg.GetInt("grpcMessageSize")
	if err != nil {
		return nil, err
	}

	prometheusEndpoint := viper.GetString("prometheusEndpoint")

	mtmConn, err := metamorph.DialGRPC(metamorphAddress, prometheusEndpoint, grpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to metamorph server: %v", err)
	}

	metamorphClient := metamorph.NewClient(metamorph_api.NewMetaMorphAPIClient(mtmConn))

	k8sClient, err := k8s_client.New()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s-client: %v", err)
	}

	namespace, err := cfg.GetString("k8sWatcher.namespace")
	if err != nil {
		return nil, err
	}

	blocktxAddress, err := cfg.GetString("blocktx.dialAddr")
	if err != nil {
		return nil, err
	}

	blocktxConn, err := blocktx.DialGRPC(blocktxAddress, prometheusEndpoint, grpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	blocktxClient := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(blocktxConn))

	k8sWatcher := k8s_watcher.New(metamorphClient, blocktxClient, k8sClient, namespace, k8s_watcher.WithLogger(logger))
	err = k8sWatcher.Start()
	if err != nil {
		return nil, fmt.Errorf("faile to start k8s-watcher: %v", err)
	}

	return func() {
		logger.Info("Shutting down K8s watcher")
		k8sWatcher.Shutdown()
	}, nil
}
