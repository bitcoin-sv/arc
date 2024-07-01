package address

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

			keysFlag := viper.GetString("keys")
			selectedKeys := strings.Split(keysFlag, ",")

			var keys map[string]string
			err = viper.UnmarshalKey("privateKeys", &keys)
			if err != nil {
				return err
			}

			var keySets []*keyset.KeySet

			if len(keys) > 0 {
				if len(selectedKeys) > 0 {
					for _, selectedKey := range selectedKeys {
						key, found := keys[selectedKey]
						if !found {
							return fmt.Errorf("key not found: %s", selectedKey)
						}
						fundingKeySet, _, err := helper.GetKeySetsXpriv(key)
						if err != nil {
							return fmt.Errorf("failed to get key sets: %v", err)
						}
						keySets = append(keySets, fundingKeySet)
					}
				} else {
					for _, key := range keys {
						fundingKeySet, _, err := helper.GetKeySetsXpriv(key)
						if err != nil {
							return fmt.Errorf("failed to get key sets: %v", err)
						}
						keySets = append(keySets, fundingKeySet)
					}
				}
			} else {
				return errors.New("no keys given in configuration")
			}

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
