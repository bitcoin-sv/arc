package app

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var addrCmd = &cobra.Command{
	Use:   "address",
	Short: "show address of the wallet",
	RunE: func(cmd *cobra.Command, args []string) error {
		keyFile := viper.GetString("keyFile")
		isTestnet := viper.GetBool("testnet")

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

		logger.Info("address", "funding key", fundingKeySet.Address(!isTestnet))

		receivingKeySet, err := keyset.NewFromExtendedKeyStr(xpriv, "0/1")
		if err != nil {
			return err
		}
		logger.Info("address", "receiving key", receivingKeySet.Address(!isTestnet))

		return nil
	},
}

func init() {
	walletCmd.AddCommand(addrCmd)
}
