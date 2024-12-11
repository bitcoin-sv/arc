package topup

import (
	"context"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

var Cmd = &cobra.Command{
	Use:   "topup",
	Short: "Top up funding address with BSV",
	RunE: func(_ *cobra.Command, _ []string) error {
		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}
		wocAPIKey, err := helper.GetString("wocAPIKey")
		if err != nil {
			return err
		}

		logger := helper.GetLogger()

		wocClient := woc_client.New(!isTestnet, woc_client.WithAuth(wocAPIKey), woc_client.WithLogger(logger))

		keySetsMap, err := helper.GetSelectedKeySets()
		if err != nil {
			return err
		}

		for keyName, keySet := range keySetsMap {
			if wocAPIKey == "" {
				time.Sleep(500 * time.Millisecond)
			}
			err = wocClient.TopUp(context.Background(), keySet.Address(!isTestnet))

			if err != nil {
				return err
			}
			logger.Info("top up complete", slog.String("address", keySet.Address(!isTestnet)), slog.String("name", keyName))
		}

		return nil
	},
}
