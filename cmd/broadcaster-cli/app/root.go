package app

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
)

var (
	RootCmd        = &cobra.Command{}
	ConfigFileName string
)

func Execute() error {

	var err error
	RootCmd.PersistentFlags().Bool("testnet", false, "Use testnet")
	err = viper.BindPFlag("testnet", RootCmd.PersistentFlags().Lookup("testnet"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.PersistentFlags().StringSlice("keys", []string{}, "List of selected private keys")
	err = viper.BindPFlag("keys", RootCmd.PersistentFlags().Lookup("keys"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.PersistentFlags().String("wocAPIKey", "", "Optional WhatsOnChain API key for allowing for higher request rates")
	err = viper.BindPFlag("wocAPIKey", RootCmd.PersistentFlags().Lookup("wocAPIKey"))
	if err != nil {
		log.Fatal(err)
	}

	RootCmd.PersistentFlags().String("config", "", "Path to the config file to be used")

	fp := filepath.Dir(ConfigFileName)
	filename := filepath.Base(ConfigFileName)
	extension := filepath.Ext(filename)

	if extension == "" {
		log.Fatal("config file extension missing")
	}

	if extension != ".yaml" {
		log.Fatalf("config file extension needs to be yaml but is %s", extension)
	}

	fmt.Println("Config name: ", filename)

	viper.AddConfigPath(".")
	viper.AddConfigPath("./cmd/broadcaster-cli/")
	if fp != "" {
		viper.AddConfigPath(fp)
	}

	viper.SetConfigName(filename[:len(filename)-len(extension)])

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

	return RootCmd.Execute()
}
