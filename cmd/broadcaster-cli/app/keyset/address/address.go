package address

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "address",
	Short: "Show address of the keyset",
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

		fundingKeySet, err := keyset.NewFromExtendedKeyStr(xpriv, "0/0")
		if err != nil {
			return err
		}

		logger.Info("address", slog.String("funding key", fundingKeySet.Address(!isTestnet)))

		receivingKeySet, err := keyset.NewFromExtendedKeyStr(xpriv, "0/1")
		if err != nil {
			return err
		}
		logger.Info("address", slog.String("receiving key", receivingKeySet.Address(!isTestnet)))

		return nil
	},
}
