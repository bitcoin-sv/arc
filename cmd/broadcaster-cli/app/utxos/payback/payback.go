package payback

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "payback",
	Short: "Pay all funds from receiving key set to funding keyset",
	RunE: func(cmd *cobra.Command, args []string) error {

		isTestnet := helper.GetBool("testnet")
		callbackURL := helper.GetString("callback")
		callbackToken := helper.GetString("callbackToken")
		authorization := helper.GetString("authorization")
		keyFile := helper.GetString("keyFile")
		miningFeeSat := helper.GetInt("miningFeeSatPerKb")
		arcServer := helper.GetString("apiURL")

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, arcServer)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		var fundingKeySet *keyset.KeySet
		var receivingKeySet *keyset.KeySet

		fundingKeySet, receivingKeySet, err = helper.GetKeySetsKeyFile(keyFile)
		if err != nil {
			return fmt.Errorf("failed to get key sets: %v", err)
		}

		wocClient := woc_client.New()

		preparer, err := broadcaster.NewRateBroadcaster(logger, client, fundingKeySet, receivingKeySet, &wocClient,
			broadcaster.WithFees(miningFeeSat),
			broadcaster.WithIsTestnet(isTestnet),
			broadcaster.WithCallback(callbackURL, callbackToken),
		)
		if err != nil {
			return fmt.Errorf("failed to create rate broadcaster: %v", err)
		}

		err = preparer.Payback()
		if err != nil {
			return fmt.Errorf("failed to submit pay back txs: %v", err)
		}
		return nil
	},
}
