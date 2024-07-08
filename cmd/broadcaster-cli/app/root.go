package app

import (
	"errors"
	"log"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var RootCmd = &cobra.Command{
	Use:   "broadcaster",
	Short: "CLI tool to broadcast transactions to ARC",
}

func init() {
	var err error
	RootCmd.PersistentFlags().Bool("testnet", false, "Use testnet")
	err = viper.BindPFlag("testnet", RootCmd.PersistentFlags().Lookup("testnet"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.PersistentFlags().String("keyfile", "", "Comma separated list of paths to private key files")
	err = viper.BindPFlag("keyFile", RootCmd.PersistentFlags().Lookup("keyfile"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.PersistentFlags().String("wocAPIKey", "", "Optional WhatsOnChain API key for allowing for higher request rates")
	err = viper.BindPFlag("wocAPIKey", RootCmd.PersistentFlags().Lookup("wocAPIKey"))
	if err != nil {
		log.Fatal(err)
	}

	viper.AddConfigPath(".")
	viper.AddConfigPath("./cmd/broadcaster-cli/")

	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	err = viper.ReadInConfig()

	var viperErr viper.ConfigFileNotFoundError
	isConfigFileNotFoundErr := errors.As(err, &viperErr)

	if err != nil && !isConfigFileNotFoundErr {
		log.Fatal(err)
	}

	RootCmd.AddCommand(keyset.Cmd)
	RootCmd.AddCommand(utxos.Cmd)
}

func Execute() error {
	return RootCmd.Execute()
}
