package topup

import (
	"context"
	"log/slog"

	"time"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "topup",
	Short: "Top up funding address with BSV",
	RunE: func(cmd *cobra.Command, args []string) error {
		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}
		wocApiKey, err := helper.GetString("wocAPIKey")
		if err != nil {
			return err
		}

		logger := helper.GetLogger()

		wocClient := woc_client.New(woc_client.WithAuth(wocApiKey), woc_client.WithLogger(logger))

		keySets, err := helper.GetKeySets()
		if err != nil {
			return err
		}

		for _, keySet := range keySets {

			if wocApiKey == "" {
				time.Sleep(500 * time.Millisecond)
			}
			err = wocClient.TopUp(context.Background(), !isTestnet, keySet.Address(!isTestnet))

			if err != nil {
				return err
			}
			logger.Info("top up complete", slog.String("address", keySet.Address(!isTestnet)))
		}

		return nil
	},
}
