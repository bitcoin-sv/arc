package app

import (
	"fmt"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/broadcast"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/prep_utxos"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "broadcaster",
		Short: "cli tool to broadcast transactions to ARC",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			v.SetConfigName("config")
			v.SetConfigType("yaml")
			v.AddConfigPath(".")
			v.AddConfigPath("../../")
			err := v.ReadInConfig()
			if err != nil {
				return fmt.Errorf("failed to read config file config.yaml: %v", err)
			}
			return nil
		},
	}

	broadcastCmd := broadcast.InitCommand(v)
	cmd.AddCommand(broadcastCmd)

	prepUTXOsCmd := prep_utxos.InitCommand(v)
	cmd.AddCommand(prepUTXOsCmd)

	return cmd
}
