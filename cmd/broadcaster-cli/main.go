package main

import (
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app"
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

	err := app.Execute()
	if err != nil {
		return err
	}

	return nil
}
