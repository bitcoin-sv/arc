package utxos

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/broadcast"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/consolidate"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/create"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/split"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

var Cmd = &cobra.Command{
	Use:   "utxos",
	Short: "Create UTXO set to be used with broadcaster",
}

func init() {
	var err error

	Cmd.PersistentFlags().String("apiURL", "", "URL of ARC api")
	err = viper.BindPFlag("apiURL", Cmd.PersistentFlags().Lookup("apiURL"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.PersistentFlags().String("authorization", "", "Authorization header to use for the http api client")
	err = viper.BindPFlag("authorization", Cmd.PersistentFlags().Lookup("authorization"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.PersistentFlags().String("callback", "", "URL which will be called with ARC callbacks")
	err = viper.BindPFlag("callback", Cmd.PersistentFlags().Lookup("callback"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.PersistentFlags().String("callbackToken", "", "Token used as authentication header to be sent with ARC callbacks")
	err = viper.BindPFlag("callbackToken", Cmd.PersistentFlags().Lookup("callbackToken"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.PersistentFlags().Int("miningFeeSatPerKb", 1, "Mining fee offered in transactions [sat/kb]")
	err = viper.BindPFlag("miningFeeSatPerKb", Cmd.PersistentFlags().Lookup("miningFeeSatPerKb"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.PersistentFlags().BoolP("fullStatusUpdates", "f", false, fmt.Sprintf("Send callbacks for %s or %s status", metamorph_api.Status_SEEN_ON_NETWORK.String(), metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL.String()))
	err = viper.BindPFlag("fullStatusUpdates", Cmd.PersistentFlags().Lookup("fullStatusUpdates"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.PersistentFlags().Int("satoshis", 0, "Nr of satoshis per output outputs")
	err = viper.BindPFlag("satoshis", Cmd.PersistentFlags().Lookup("satoshis"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.AddCommand(create.Cmd)
	Cmd.AddCommand(broadcast.Cmd)
	Cmd.AddCommand(consolidate.Cmd)
	Cmd.AddCommand(split.Cmd)
}
