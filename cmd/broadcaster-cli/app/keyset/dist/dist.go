package dist

import (
	"errors"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"os"
	"strings"

	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "dist",
	Short: "show distribution of utxo sizes in key set",
	RunE: func(cmd *cobra.Command, args []string) error {
		keyFile := viper.GetString("keyFile")
		isTestnet := viper.GetBool("testnet")

		extendedBytes, err := os.ReadFile(keyFile)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("arc.key not found. Please create this file with the xpriv you want to use")
			}
			return err
		}
		xpriv := strings.TrimRight(strings.TrimSpace((string)(extendedBytes)), "\n")

		wocClient := woc_client.New()

		fundingKeySet, err := keyset.NewFromExtendedKeyStr(xpriv, "0/0")
		if err != nil {
			return err
		}
		_, err = wocClient.GetUTXOs(!isTestnet, fundingKeySet.Script, fundingKeySet.Address(!isTestnet))
		if err != nil {
			return err
		}

		// todo: draw historgram of utxo satoshi sizes

		return nil
	},
}
