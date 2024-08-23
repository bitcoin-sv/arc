package utxos

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

var Cmd = &cobra.Command{
	Use:   "utxos",
	Short: "Show distribution of utxo sizes in key set",
	RunE: func(cmd *cobra.Command, args []string) error {
		maxRows := viper.GetInt("rows")

		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}
		wocApiKey, err := helper.GetString("wocAPIKey")
		if err != nil {
			return err
		}

		logger := helper.GetLogger()
		wocClient := woc_client.New(!isTestnet, woc_client.WithAuth(wocApiKey), woc_client.WithLogger(logger))

		keySets, err := helper.GetKeySets()
		if err != nil {
			return err
		}

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt) // Listen for Ctrl+C

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			<-signalChan
			cancel()
		}()

		names := helper.GetOrderedKeys(keySets)
		counter := 0
		var t table.Writer
		ksRow := map[string]*keyset.KeySet{}
		for _, name := range names {
			ksRow[name] = keySets[name]
			if counter >= 9 {
				t = table.NewWriter()
				t := getUtxosTable(ctx, logger, t, ksRow, isTestnet, wocClient, maxRows)
				t.SetStyle(table.StyleColoredBright)
				fmt.Println(t.Render())
				fmt.Println()
				ksRow = map[string]*keyset.KeySet{}
				counter = 0
				continue
			}
			counter++
		}
		if len(ksRow) > 0 {
			t = table.NewWriter()
			t := getUtxosTable(ctx, logger, t, ksRow, isTestnet, wocClient, maxRows)
			t.SetStyle(table.StyleColoredBright)
			fmt.Println(t.Render())
		}

		return nil
	},
}

func init() {
	var err error

	Cmd.Flags().IntP("rows", "r", 0, "Maximum rows to show - default: all")
	err = viper.BindPFlag("rows", Cmd.Flags().Lookup("rows"))
	if err != nil {
		log.Fatal(err)
	}
}
