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

			keySets, err := helper.GetKeySets()
			if err != nil {
				return err
			}

			names := helper.GetOrderedKeys(keySets)

			for _, name := range names {
				keySet := keySets[name]

				logger.Info("address", slog.String("name", name), slog.String("address", keySet.Address(!isTestnet)), slog.String("key", keySet.GetMaster().String()))
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
