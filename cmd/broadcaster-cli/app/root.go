package app

import (
	"log"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
)

var (
	RootCmd = &cobra.Command{
		Use:   "broadcaster",
		Short: "CLI tool to broadcast transactions to ARC",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			logLevel := helper.GetString("logLevel")
			logFormat := helper.GetString("logFormat")
			logger := helper.NewLogger(logLevel, logFormat)
			if viper.ConfigFileUsed() != "" {
				logger.Info("config file used", slog.String("filename", viper.ConfigFileUsed()))
			}
		},
	}
)

func init() {
	logger := log.Default()
	var err error
	RootCmd.PersistentFlags().Bool("testnet", false, "Use testnet")
	err = viper.BindPFlag("testnet", RootCmd.PersistentFlags().Lookup("testnet"))
	if err != nil {
		logger.Printf("failed to bind flag testnet: %w", err)
		os.Exit(1)
	}

	RootCmd.PersistentFlags().StringSlice("keys", []string{}, "List of selected private keys")
	err = viper.BindPFlag("keys", RootCmd.PersistentFlags().Lookup("keys"))
	if err != nil {
		logger.Printf("failed to bind flag keys: %w", err)
		os.Exit(1)
	}

	RootCmd.PersistentFlags().String("wocAPIKey", "", "Optional WhatsOnChain API key for allowing for higher request rates")
	err = viper.BindPFlag("wocAPIKey", RootCmd.PersistentFlags().Lookup("wocAPIKey"))
	if err != nil {
		logger.Printf("failed to bind flag wocAPIKey: %w", err)
		os.Exit(1)
	}

	RootCmd.PersistentFlags().String("logLevel", "INFO", "mode of logging. Value can be one of TRACE | DEBUG | INFO | WARN | ERROR")
	err = viper.BindPFlag("logLevel", RootCmd.PersistentFlags().Lookup("logLevel"))
	if err != nil {
		logger.Printf("failed to bind flag logLevel: %w", err)
		os.Exit(1)
	}

	RootCmd.PersistentFlags().String("logFormat", "text", "format of logging. Value can be one of text | json | tint")
	err = viper.BindPFlag("logFormat", RootCmd.PersistentFlags().Lookup("logFormat"))
	if err != nil {
		logger.Printf("failed to bind flag logFormat: %w", err)
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
		logger.Printf("failed to read config file: %w", err)
		os.Exit(1)
	}

	RootCmd.AddCommand(keyset.Cmd)
	RootCmd.AddCommand(utxos.Cmd)
}

func Execute() error {
	return RootCmd.Execute()
}
