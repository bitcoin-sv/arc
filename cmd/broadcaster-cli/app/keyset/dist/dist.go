package dist

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "dist",
	Short: "Show distribution of utxo sizes in key set",
	RunE: func(cmd *cobra.Command, args []string) error {
		keyFile, err := helper.GetString("keyFile")
		if err != nil {
			return err
		}
		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}

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
		utxos, err := wocClient.GetUTXOs(!isTestnet, fundingKeySet.Script, fundingKeySet.Address(!isTestnet))
		if err != nil {
			return err
		}

		valuesMap := map[uint64]int{}
		keysSlice := []uint64{}
		var found bool
		for _, utxo := range utxos {
			_, found = valuesMap[utxo.Satoshis]
			if found {
				valuesMap[utxo.Satoshis]++
				continue
			}

			valuesMap[utxo.Satoshis] = 1

			keysSlice = append(keysSlice, utxo.Satoshis)
		}

		sort.Slice(keysSlice, func(i, j int) bool {
			return keysSlice[j] < keysSlice[i]
		})

		t := table.NewWriter()
		fmt.Printf("Distribution of satoshis for address %s\n", fundingKeySet.Address(!isTestnet))

		t.AppendHeader(table.Row{"Satoshis", "Outputs"})
		for _, satoshi := range keysSlice {
			t.AppendRow(table.Row{strconv.FormatUint(satoshi, 10), strconv.Itoa(valuesMap[satoshi])})
		}
		fmt.Println(t.Render())

		return nil
	},
}
