package address

import (
	"log/slog"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/spf13/cobra"
)

var (
	logger *slog.Logger
	Cmd    = &cobra.Command{
		Use:   "address",
		Short: "Show address of the keyset",
		RunE: func(cmd *cobra.Command, args []string) error {

			isTestnet, err := helper.GetBool("testnet")
			if err != nil {
				return err
			}

			// keysFlag := viper.GetString("keys")
			// selectedKeys := strings.Split(keysFlag, ",")

			keySets, err := helper.GetKeySets()
			if err != nil {
				return err
			}

			// keys, err := helper.GetPrivateKeys()
			// if err != nil {
			// 	return err
			// }

			// keySets, err := helper.GetKeySets(keys, selectedKeys)
			// if err != nil {
			// 	return err
			// }

			for _, keySet := range keySets {

				logger.Info("address", slog.String("address", keySet.Address(!isTestnet)))
			}

			return nil
		},
	}
)

func init() {
	logger = helper.GetLogger()

	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide unused persistent flags
		err := command.Flags().MarkHidden("wocAPIKey")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}
		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})
}
