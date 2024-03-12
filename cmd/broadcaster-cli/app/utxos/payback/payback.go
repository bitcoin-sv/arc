package payback

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "payback",
	Short: "Pay all funds from receiving key set to funding keyset",
	RunE: func(cmd *cobra.Command, args []string) error {

		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}
		callbackURL, err := helper.GetString("callback")
		if err != nil {
			return err
		}
		callbackToken, err := helper.GetString("callbackToken")
		if err != nil {
			return err
		}
		authorization, err := helper.GetString("authorization")
		if err != nil {
			return err
		}
		keyFile, err := helper.GetString("keyFile")
		if err != nil {
			return err
		}
		miningFeeSat, err := helper.GetInt("miningFeeSatPerKb")
		if err != nil {
			return err
		}
		arcServer, err := helper.GetString("apiURL")
		if err != nil {
			return err
		}

		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))

		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, arcServer)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		fundingKeySet, receivingKeySet, err := helper.GetKeySetsKeyFile(keyFile)
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
