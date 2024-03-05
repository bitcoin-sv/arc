package utxos

import (
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/create"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/payback"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
)

var UtxosCmd = &cobra.Command{
	Use:   "utxos",
	Short: "Create UTXO set to be used with broadcaster",
}

func init() {
	var err error

	UtxosCmd.PersistentFlags().String("api-url", "", "Send all funds from receiving key set to funding key set")
	err = viper.BindPFlag("api-url", UtxosCmd.PersistentFlags().Lookup("api-url"))
	if err != nil {
		log.Fatal(err)
	}

	UtxosCmd.AddCommand(payback.PaybackCmd)
	UtxosCmd.AddCommand(create.CreateCmd)
}
