package cmd

import (
	"github.com/bitcoin-sv/arc/k8s_coordinator"
	"github.com/bitcoin-sv/arc/k8s_coordinator/k8s_client"

	"github.com/ordishs/go-utils"
)

func StartK8sCoordinator(logger utils.Logger) (func(), error) {
	k8sClient, err := k8s_client.New()

	if err != nil {
		return nil, err
	}

	k8sCoordinator := k8s_coordinator.New()
}
