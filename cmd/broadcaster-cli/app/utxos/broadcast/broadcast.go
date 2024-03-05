package broadcast

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

var Cmd = &cobra.Command{
	Use:   "broadcast",
	Short: "submit transactions to ARC",
	RunE: func(cmd *cobra.Command, args []string) error {

		outputs := viper.GetInt("outputs")
		satoshisPerOutput := viper.GetUint64("satoshis")
		rate := viper.GetInt("rate")

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

		preparer, err := broadcaster.NewRateBroadcaster(logger, client, fundingKeySet, receivingKeySet, &wocClient,
			broadcaster.WithFees(miningFeeSat),
			broadcaster.WithIsTestnet(isTestnet),
			broadcaster.WithCallbackURL(callbackURL),
		)
		if err != nil {
			return fmt.Errorf("failed to create rate broadcaster: %v", err)
		}

		err = preparer.Broadcast(outputs, satoshisPerOutput, rate)
		if err != nil {
			return fmt.Errorf("failed to broadcast back txs: %v", err)
		}
		return nil
	},
}

func init() {
	var err error

	Cmd.Flags().Int("rate", 10, "transactions per second to be rate broadcasted")
	err = viper.BindPFlag("rate", Cmd.Flags().Lookup("rate"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("outputs", 10, "Nr of requested outputs")
	err = viper.BindPFlag("outputs", Cmd.Flags().Lookup("outputs"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("satoshis", 1000, "Nr of satoshis per output outputs")
	err = viper.BindPFlag("satoshis", Cmd.Flags().Lookup("satoshis"))
	if err != nil {
		log.Fatal(err)
	}
}
