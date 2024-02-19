package prep_utxos

import (
	"errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare-utxos",
		Short: "Create UTXO set to be used with broadcaster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented")
		},
	}

	return cmd
}
