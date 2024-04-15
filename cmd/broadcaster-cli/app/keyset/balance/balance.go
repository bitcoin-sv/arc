package balance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/woc_client"
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
		if keyFile == "" {
			return errors.New("no key file given")
		}

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

		keyFiles := strings.Split(keyFile, ",")

		for _, kf := range keyFiles {
			fundingKeySet, _, err := helper.GetKeySetsKeyFile(kf)
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}

			if wocApiKey == "" {
				time.Sleep(500 * time.Millisecond)
			}
			confirmed, unconfirmed, err := wocClient.GetBalanceWithRetries(context.Background(), !isTestnet, fundingKeySet.Address(!isTestnet), 1*time.Second, 5)
			if err != nil {
				return err
			}
			logger.Info("balance", slog.String("keyfile", kf), slog.String("address", fundingKeySet.Address(!isTestnet)), slog.Int64("confirmed", confirmed), slog.Int64("unconfirmed", unconfirmed))
		}

		return nil
	},
}
