package balance

import (
	"fmt"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
	"log/slog"
	"os"
)

var Cmd = &cobra.Command{
	Use:   "balance",
	Short: "Show balance of the keyset",
	RunE: func(cmd *cobra.Command, args []string) error {
		keyFile, err := helper.GetString("keyFile")
		if err != nil {
			return err
		}
		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}

		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))

		wocClient := woc_client.New()
		fundingKeySet, receivingKeySet, err := helper.GetKeySetsKeyFile(keyFile)
		if err != nil {
			return fmt.Errorf("failed to get key sets: %v", err)
		}

		fundingBalance, err := wocClient.GetBalance(!isTestnet, fundingKeySet.Address(!isTestnet))
		if err != nil {
			return err
		}
		logger.Info("balance", slog.Int64("funding key", fundingBalance))

		receivingBalance, err := wocClient.GetBalance(!isTestnet, receivingKeySet.Address(!isTestnet))
		if err != nil {
			return err
		}
		logger.Info("balance", slog.Int64("receiving key", receivingBalance))

		return nil
	},
}
