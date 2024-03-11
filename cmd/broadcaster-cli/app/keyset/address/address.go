package address

import (
	"fmt"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
	"log/slog"
	"os"
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
		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))

		fundingKeySet, receivingKeySet, err := helper.GetKeySetsKeyFile(keyFile)
		if err != nil {
			return fmt.Errorf("failed to get key sets: %v", err)
		}

		logger.Info("address", slog.String("funding key", fundingKeySet.Address(!isTestnet)))

		logger.Info("address", slog.String("receiving key", receivingKeySet.Address(!isTestnet)))

		return nil
	},
}
