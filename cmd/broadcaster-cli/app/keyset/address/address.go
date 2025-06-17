package address

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
)

var (
	logger *slog.Logger
	Cmd    = &cobra.Command{
		Use:   "address",
		Short: "Show address of the keyset",
		RunE: func(_ *cobra.Command, _ []string) error {
			isTestnet, err := helper.GetBool("testnet")
			if err != nil {
				return err
			}

			keySetsMap, err := helper.GetSelectedKeySets()
			if err != nil {
				return err
			}

			names := helper.GetOrderedKeys(keySetsMap)

			//logLevel := helper.GetString("logLevel")
			//logFormat := helper.GetString("logFormat")
			//logger := helper.NewLogger(logLevel, logFormat)
			for _, name := range names {
				keySet := keySetsMap[name]

				logger.Info("address", slog.String("name", name), slog.String("address", keySet.Address(!isTestnet)), slog.String("key", keySet.GetMaster().String()))
			}

			return nil
		},
	}
)

func init() {
	logLevel := helper.GetString("logLevel")
	logFormat := helper.GetString("logFormat")
	logger = helper.NewLogger(logLevel, logFormat)
	//logger := log.Default()

	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide unused persistent flags
		err := command.Flags().MarkHidden("wocAPIKey")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("error", err.Error()))
			//logger.Printf("failed to mark flag hidden: %v", err)
		}
		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})
}
