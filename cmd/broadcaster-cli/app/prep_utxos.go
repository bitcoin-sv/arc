package app

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var prepCmd = &cobra.Command{
	Use:   "prepare-utxos",
	Short: "Create UTXO set to be used with broadcaster",
	RunE: func(cmd *cobra.Command, args []string) error {

		isTestnet := viper.GetBool("testnet")
		isPayback := viper.GetBool("payback")
		callbackURL := viper.GetString("callback")
		authorization := viper.GetString("authorization")
		keyFile := viper.GetString("keyFile")

		miningFeeSat := viper.GetInt("broadcaster.miningFeeSatPerKb")

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		var client broadcaster.ArcClient
		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, false, true)
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

		preparer := broadcaster.NewUTXOPreparer(logger, client, fundingKeySet, receivingKeySet, &wocClient,
			broadcaster.WithFees(miningFeeSat),
			broadcaster.WithIsTestnet(isTestnet),
			broadcaster.WithCallbackURL(callbackURL),
		)

		if isPayback {
			err := preparer.Payback()
			if err != nil {
				return fmt.Errorf("failed to submit pay back txs: %v", err)
			}
			return nil
		}

		return errors.New("prepare-utxos functionality not yet implemented")
	},
}

func init() {
	var err error

	prepCmd.Flags().Bool("payback", false, "Send all funds from receiving key set to funding key set")
	err = viper.BindPFlag("payback", prepCmd.Flags().Lookup("payback"))
	if err != nil {
		log.Fatal(err)
	}

	prepCmd.Flags().String("api-url", "", "Send all funds from receiving key set to funding key set")
	err = viper.BindPFlag("api-url", prepCmd.Flags().Lookup("api-url"))
	if err != nil {
		log.Fatal(err)
	}
	rootCmd.AddCommand(prepCmd)
}
