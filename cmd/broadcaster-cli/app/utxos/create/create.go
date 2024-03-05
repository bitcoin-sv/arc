package create

import (
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

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a UTXO set",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputs := viper.GetInt("outputs")
		satoshisPerOutput := viper.GetUint64("satoshis")

		isTestnet := viper.GetBool("testnet")
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

		_, err = preparer.CreateUtxos(outputs, satoshisPerOutput)
		if err != nil {
			return fmt.Errorf("failed to submit pay back txs: %v", err)
		}
		return nil
	},
}

func init() {
	var err error

	CreateCmd.Flags().Int("outputs", 10, "Nr of requested outputs")
	err = viper.BindPFlag("outputs", CreateCmd.Flags().Lookup("outputs"))
	if err != nil {
		log.Fatal(err)
	}

	CreateCmd.Flags().Int("satoshis", 1000, "Nr of satoshis per output outputs")
	err = viper.BindPFlag("satoshis", CreateCmd.Flags().Lookup("satoshis"))
	if err != nil {
		log.Fatal(err)
	}
}
