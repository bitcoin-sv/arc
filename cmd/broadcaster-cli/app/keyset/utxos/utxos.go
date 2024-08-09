package utxos

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

		t := getUtxosTable(ctx, logger, keySets, isTestnet, wocClient, maxRows)
		fmt.Println(t.Render())

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
