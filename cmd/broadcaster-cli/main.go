package main

import (
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app"
	"github.com/spf13/viper"
	"log"
	"os"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("failed to run broadcaster: %v", err)
	}

	os.Exit(0)
}

func run() error {
	v := viper.GetViper()

	rootCmd := app.InitCommand(v)

	err := rootCmd.Execute()
	if err != nil {
		return err
	}

	return nil
}
