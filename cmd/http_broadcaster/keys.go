package main

import (
	"fmt"
	"os"
	"strings"
)

var (
	apiKey string
	xpriv  string
)

func init() {
	var err error

	xpriv, err = readFile("arc.key")
	if err != nil {
		panic(err)
	}

	apiKey, err = readFile("api.key")
	if err != nil {
		panic(err)
	}
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %q: %w", path, err)
	}

	// Remove whitespace and trailing newline
	return strings.TrimRight(strings.TrimSpace((string)(b)), "\n"), nil
}
