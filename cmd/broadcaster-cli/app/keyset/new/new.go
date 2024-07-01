package new

import (
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/spf13/cobra"
)

var (
	logger *slog.Logger
	Cmd    = &cobra.Command{
		Use:   "new",
		Short: "Create new key set",
		RunE: func(cmd *cobra.Command, args []string) error {

			newKeyset, err := keyset.New()
			if err != nil {
				return err
			}

			fmt.Println(newKeyset.GetMaster().String())
			return nil
		},
	}
)

func init() {
	var err error

	logger = helper.GetLogger()

	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide unused persistent flags
		err = command.Flags().MarkHidden("testnet")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}
		err = command.Flags().MarkHidden("keyfile")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}
		err = command.Flags().MarkHidden("wocAPIKey")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}
		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})
}
