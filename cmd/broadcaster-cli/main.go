package main

import (
	"log"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("failed to run broadcaster-cli: %w", err)
	}
}

func run() error {
	err := app.Execute()
	if err != nil {
		return err
	}

	return nil
}
