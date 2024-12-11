package balance

import (
	"context"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

var Cmd = &cobra.Command{
	Use:   "balance",
	Short: "Show balance of the keyset",
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

		names := helper.GetOrderedKeys(keySetsMap)

		for _, name := range names {
			keySet := keySetsMap[name]
			if wocAPIKey == "" {
				time.Sleep(500 * time.Millisecond)
			}
			confirmed, unconfirmed, err := wocClient.GetBalanceWithRetries(context.Background(), keySet.Address(!isTestnet), 1*time.Second, 5)
			if err != nil {
				return err
			}
			logger.Info("balance", slog.String("name", name), slog.String("address", keySet.Address(!isTestnet)), slog.Int64("confirmed", confirmed), slog.Int64("unconfirmed", unconfirmed))
		}

		return nil
	},
}
