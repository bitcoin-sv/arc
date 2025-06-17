package new

import (
	"log/slog"

	chaincfg "github.com/bsv-blockchain/go-sdk/transaction/chaincfg"
	"github.com/spf13/cobra"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

var (
	logger *slog.Logger
	Cmd    = &cobra.Command{
		Use:   "new",
		Short: "Create new key set",
		RunE: func(_ *cobra.Command, _ []string) error {
			isTestnet, err := helper.GetBool("testnet")
			if err != nil {
				return err
			}

			netCfg := chaincfg.MainNet
			if isTestnet {
				netCfg = chaincfg.TestNet
			}

			newKeyset, err := keyset.New(&netCfg)
			if err != nil {
				return err
			}

			logger.Info("new keyset", slog.String("keyset", newKeyset.GetMaster().String()))
			return nil
		},
	}
)

func init() {
	var err error

	logLevel := helper.GetString("logLevel")
	logFormat := helper.GetString("logFormat")
	logger = helper.NewLogger(logLevel, logFormat)

	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide unused persistent flags
		err = command.Flags().MarkHidden("keyfile")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("flag", "keyfile"), slog.String("err", err.Error()))
		}
		err = command.Flags().MarkHidden("wocAPIKey")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("flag", "wocAPIKey"), slog.String("err", err.Error()))
		}
		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})
}
