package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	RootCmd = &cobra.Command{
		Use:   "broadcaster",
		Short: "CLI tool to broadcast transactions to ARC",
	}
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("failed to run broadcaster-cli: %v", err)
	}

	os.Exit(0)
}

func run() error {
	var err error

	RootCmd.PersistentFlags().Bool("testnet", false, "[IS NOT DISPLAYED]")
	RootCmd.PersistentFlags().StringSlice("keys", []string{}, "[IS NOT DISPLAYED]")
	RootCmd.PersistentFlags().String("wocAPIKey", "", "[IS NOT DISPLAYED]")

	RootCmd.PersistentFlags().String("config", "broadcaster-cli", "[IS NOT DISPLAYED]")
	err = viper.BindPFlag("config", RootCmd.PersistentFlags().Lookup("config"))
	if err != nil {
		log.Fatalf("failed to get config: %v", err)
	}

	err = RootCmd.Execute()
	if err != nil {
		return fmt.Errorf("failed to execute root command: %w", err)
	}

	app.ConfigFileName = viper.GetString("config")
	app.RootCmd.Use = RootCmd.Use
	app.RootCmd.Short = RootCmd.Short

	err = app.Execute()
	if err != nil {
		return fmt.Errorf("failed to execute application command: %w", err)
	}

	return nil
}
