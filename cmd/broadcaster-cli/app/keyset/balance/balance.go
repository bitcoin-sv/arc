package balance

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/lmittmann/tint"
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
		wocApiKey, err := helper.GetString("wocAPIKey")
		if err != nil {
			return err
		}

		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))

		wocClient := woc_client.New(woc_client.WithAuth(wocApiKey))

		keyFiles := strings.Split(keyFile, ",")

		for _, kf := range keyFiles {
			fundingKeySet, _, err := helper.GetKeySetsKeyFile(kf)
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}

			if wocApiKey == "" {
				time.Sleep(500 * time.Millisecond)
			}
			fundingBalance, err := wocClient.GetBalance(!isTestnet, fundingKeySet.Address(!isTestnet))
			if err != nil {
				return err
			}
			logger.Info("balance", slog.Int64(fundingKeySet.Address(!isTestnet), fundingBalance))
		}

		return nil
	},
}
