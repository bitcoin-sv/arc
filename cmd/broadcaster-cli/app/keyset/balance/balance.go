package balance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "balance",
	Short: "Show balance of the keyset",
	RunE: func(cmd *cobra.Command, args []string) error {

		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}
		wocApiKey, err := helper.GetString("wocAPIKey")
		if err != nil {
			return err
		}

		logger := helper.GetLogger()

		wocClient := woc_client.New(woc_client.WithAuth(wocApiKey), woc_client.WithLogger(logger))

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

			if wocApiKey == "" {
				time.Sleep(500 * time.Millisecond)
			}
			confirmed, unconfirmed, err := wocClient.GetBalanceWithRetries(context.Background(), !isTestnet, keySet.Address(!isTestnet), 1*time.Second, 5)
			if err != nil {
				return err
			}
			logger.Info("balance", slog.String("address", keySet.Address(!isTestnet)), slog.Int64("confirmed", confirmed), slog.Int64("unconfirmed", unconfirmed))
		}

		return nil
	},
}
