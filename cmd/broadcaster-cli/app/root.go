package app

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
)

var (
	logger  *slog.Logger
	RootCmd = &cobra.Command{
		Use:   "broadcaster",
		Short: "CLI tool to broadcast transactions to ARC",
	}
)

func init() {
	logger = helper.GetLogger()
	var err error
	RootCmd.PersistentFlags().Bool("testnet", false, "Use testnet")
	err = viper.BindPFlag("testnet", RootCmd.PersistentFlags().Lookup("testnet"))
	if err != nil {
		logger.Error("failed to bind flag testnet", slog.String("err", err.Error()))
		os.Exit(1)
	}

	RootCmd.PersistentFlags().StringSlice("keys", []string{}, "List of selected private keys")
	err = viper.BindPFlag("keys", RootCmd.PersistentFlags().Lookup("keys"))
	if err != nil {
		logger.Error("failed to bind flag keys", slog.String("err", err.Error()))
		os.Exit(1)
	}

	RootCmd.PersistentFlags().String("wocAPIKey", "", "Optional WhatsOnChain API key for allowing for higher request rates")
	err = viper.BindPFlag("wocAPIKey", RootCmd.PersistentFlags().Lookup("wocAPIKey"))
	if err != nil {
		logger.Error("failed to bind flag wocAPIKey", slog.String("err", err.Error()))
		os.Exit(1)
	}

	var configFilenameArg string
	args := os.Args
	for i, arg := range args {
		if arg == "-c" || arg == "--config" {
			configFilenameArg = args[i+1]
			break
		}
	}

	if configFilenameArg != "" {
		viper.SetConfigFile(configFilenameArg)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("./cmd/broadcaster-cli/")
		viper.SetConfigName("broadcaster-cli")
	}

	err = viper.ReadInConfig()
	if err != nil {
		logger.Error("failed to read config file", slog.String("err", err.Error()))
		os.Exit(1)
	}

	if viper.ConfigFileUsed() != "" {
		logger.Info("Config file used", slog.String("filename", viper.ConfigFileUsed()))
	}

	RootCmd.AddCommand(keyset.Cmd)
	RootCmd.AddCommand(utxos.Cmd)
}

func Execute() error {
	return RootCmd.Execute()
}
