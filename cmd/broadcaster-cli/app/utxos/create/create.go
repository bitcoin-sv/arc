package create

import (
	"errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
)

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a UTXO set",
	RunE: func(cmd *cobra.Command, args []string) error {

		return errors.New("create-utxos functionality not yet implemented")
	},
}

func init() {
	var err error

	CreateCmd.Flags().String("api-url", "", "Send all funds from receiving key set to funding key set")
	err = viper.BindPFlag("api-url", CreateCmd.Flags().Lookup("api-url"))
	if err != nil {
		log.Fatal(err)
	}
}
