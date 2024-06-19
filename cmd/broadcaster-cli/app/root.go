package app

import (
	"errors"
	"log"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
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

	RootCmd.PersistentFlags().String("keys", "", "Comma separated list of private keys")
	err = viper.BindPFlag("keys", RootCmd.PersistentFlags().Lookup("keys"))
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
	viper.SetConfigName("broadcaster-cli")
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("failed to read config file: %v", err)
	}

	var viperErr viper.ConfigFileNotFoundError
	isConfigFileNotFoundErr := errors.As(err, &viperErr)

	if err != nil && !errors.Is(err, unix.ENOENT) && !isConfigFileNotFoundErr {
		log.Fatal(err)
	}

	RootCmd.AddCommand(keyset.Cmd)
	RootCmd.AddCommand(utxos.Cmd)
}

func Execute() error {
	return RootCmd.Execute()
}
