package dist

import (
	"fmt"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"sort"
	"strconv"
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

		wocClient := woc_client.New()

		fundingKeySet, _, err := helper.GetKeySetsKeyFile(keyFile)
		if err != nil {
			return fmt.Errorf("failed to get key sets: %v", err)
		}

		if err != nil {
			return err
		}
		utxos, err := wocClient.GetUTXOs(!isTestnet, fundingKeySet.Script, fundingKeySet.Address(!isTestnet))
		if err != nil {
			return fmt.Errorf("failed to get utxos from WoC: %v", err)
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

		fmt.Printf("Distribution of satoshis for address %s\n", fundingKeySet.Address(!isTestnet))

		t := table.NewWriter()
		t.AppendHeader(table.Row{"Satoshis", "Outputs"})
		for _, satoshi := range keysSlice {
			t.AppendRow(table.Row{strconv.FormatUint(satoshi, 10), strconv.Itoa(valuesMap[satoshi])})
		}
		fmt.Println(t.Render())

		return nil
	},
}
