package topup

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "topup",
	Short: "Top up funding address with BSV",
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
		fundingKeySet, _, err := helper.GetKeySetsKeyFile(keyFile)
		if err != nil {
			return fmt.Errorf("failed to get key sets: %v", err)
		}

		err = wocClient.TopUp(!isTestnet, fundingKeySet.Address(!isTestnet))
		if err != nil {
			return err
		}
		logger.Info("top up complete", slog.String("address", fundingKeySet.Address(!isTestnet)))

		return nil
	},
}
