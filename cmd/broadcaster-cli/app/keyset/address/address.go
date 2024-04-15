package address

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/spf13/cobra"
)

var (
	logger *slog.Logger
	Cmd    = &cobra.Command{
		Use:   "address",
		Short: "Show address of the keyset",
		RunE: func(cmd *cobra.Command, args []string) error {
			keyFile, err := helper.GetString("keyFile")
			if err != nil {
				return err
			}
			if keyFile == "" {
				return errors.New("no key file given")
			}

			isTestnet, err := helper.GetBool("testnet")
			if err != nil {
				return err
			}

			keyFiles := strings.Split(keyFile, ",")

			for _, kf := range keyFiles {
				fundingKeySet, _, err := helper.GetKeySetsKeyFile(kf)
				if err != nil {
					return fmt.Errorf("failed to get key sets: %v", err)
				}

				logger.Info("address", slog.String(kf, fundingKeySet.Address(!isTestnet)))
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
