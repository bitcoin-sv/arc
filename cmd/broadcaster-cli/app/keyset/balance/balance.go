package balance

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"github.com/spf13/cobra"
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

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		extendedBytes, err := os.ReadFile(keyFile)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("arc.key not found. Please create this file with the xpriv you want to use")
			}
			return err
		}
		xpriv := strings.TrimRight(strings.TrimSpace((string)(extendedBytes)), "\n")

		wocClient := woc_client.New()

		fundingKeySet, err := keyset.NewFromExtendedKeyStr(xpriv, "0/0")
		if err != nil {
			return err
		}
		fundingBalance, err := wocClient.GetBalance(!isTestnet, fundingKeySet.Address(!isTestnet))
		if err != nil {
			return err
		}

		logger.Info("balance", slog.Int64("funding key", fundingBalance))

		receivingKeySet, err := keyset.NewFromExtendedKeyStr(xpriv, "0/1")
		if err != nil {
			return err
		}
		receivingBalance, err := wocClient.GetBalance(!isTestnet, receivingKeySet.Address(!isTestnet))
		if err != nil {
			return err
		}
		logger.Info("balance", slog.Int64("receiving key", receivingBalance))

		return nil
	},
}
