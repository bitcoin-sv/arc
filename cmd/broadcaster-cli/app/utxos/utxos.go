package utxos

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/broadcast"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/create"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/payback"
)

var Cmd = &cobra.Command{
	Use:   "utxos",
	Short: "Create UTXO set to be used with broadcaster",
}

func init() {
	var err error

	Cmd.PersistentFlags().String("apiURL", "", "Send all funds from receiving key set to funding key set")
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

	Cmd.AddCommand(payback.Cmd)
	Cmd.AddCommand(create.Cmd)
	Cmd.AddCommand(broadcast.Cmd)
}
