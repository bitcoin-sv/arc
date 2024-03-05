package app

import (
	"fmt"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/broadcast"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
)

var RootCmd = &cobra.Command{
	Use:   "broadcaster",
	Short: "cli tool to broadcast transactions to ARC",
}

func init() {
	var err error
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().Bool("testnet", false, "Use testnet")
	err = viper.BindPFlag("testnet", RootCmd.PersistentFlags().Lookup("testnet"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.PersistentFlags().String("authorization", "", "Authorization header to use for the http api client")
	err = viper.BindPFlag("authorization", RootCmd.PersistentFlags().Lookup("authorization"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.PersistentFlags().String("callback", "", "URL which will be called with ARC callbacks")
	err = viper.BindPFlag("callback", RootCmd.PersistentFlags().Lookup("callback"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.PersistentFlags().String("keyfile", "", "private key from file (arc.key) to use for funding transactions")
	err = viper.BindPFlag("keyFile", RootCmd.PersistentFlags().Lookup("keyfile"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.AddCommand(broadcast.BroadcastCmd)
	RootCmd.AddCommand(keyset.WalletCmd)
	RootCmd.AddCommand(utxos.UtxosCmd)
}

func Execute() error {
	return RootCmd.Execute()
}

func initConfig() {

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("failed to read config file: %v\n", err)
	}
}
