package cmd

import (
	"fmt"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/k8s_coordinator"
	"github.com/bitcoin-sv/arc/k8s_coordinator/k8s_client"
	"github.com/spf13/viper"

	"github.com/ordishs/go-utils"
)

func StartK8sCoordinator(logger utils.Logger) (func(), error) {

	addresses := viper.GetString("metamorph.dialAddr")
	if addresses == "" {
		return nil, fmt.Errorf("metamorph.dialAddr not found in config")
	}

	blocktxAddress := viper.GetString("blocktx.dialAddr")
	if blocktxAddress == "" {
		return nil, fmt.Errorf("blocktx.dialAddr not found in config")
	}

	blockTxLogger, err := config.NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to create new logger: %v", err)
	}

	conn, err := blocktx.DialGRPC(blocktxAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to block-tx server: %v", err)
	}

	bTx := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(conn), blocktx.WithLogger(blockTxLogger))

	grpcMessageSize := viper.GetInt("grpcMessageSize")
	if grpcMessageSize == 0 {
		return nil, fmt.Errorf("grpcMessageSize not found in config")
	}

	txHandler, err := transactionHandler.NewMetamorph(addresses, bTx, grpcMessageSize, logger)
	if err != nil {
		return nil, err
	}

	k8sClient, err := k8s_client.New()
	if err != nil {
		return nil, err
	}

	k8sCoordinator := k8s_coordinator.New(txHandler, k8sClient, "arc-testnet")
	if err != nil {
		return nil, err
	}
}
