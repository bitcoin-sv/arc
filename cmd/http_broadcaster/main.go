package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/TAAL-GmbH/arc/lib/keyset"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
)

func main() {
	tx, err := CreateTransaction()
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return
	}

	fmt.Printf("Transaction ID: %s\n\n", tx.TxID())
	fmt.Printf("Transaction: %s\n\n", tx.String())

	if err := SendTransactionToArc(tx); err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return
	}
}

func CreateTransaction() (*bt.Tx, error) {
	// Create a keyset from the extended private key...
	keySet, err := keyset.NewFromExtendedKeyStr(xpriv, "0/0")
	if err != nil {
		return nil, fmt.Errorf("failed to create keyset: %w", err)
	}

	utxos, err := keySet.GetUTXOs(true) // true = mainnet
	if err != nil {
		return nil, fmt.Errorf("failed to get UTXOs: %w", err)
	}

	tx := bt.NewTx()

	if err := tx.FromUTXOs(utxos...); err != nil {
		return nil, fmt.Errorf("failed to add UTXOs to transaction: %w", err)
	}

	// Send all change to the same keySet script...
	if err := tx.Change(keySet.Script, feeQuote); err != nil {
		return nil, fmt.Errorf("failed to add change to transaction: %w", err)
	}

	// Sign the transaction...
	unlockerGetter := unlocker.Getter{PrivateKey: keySet.PrivateKey}
	if err = tx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return tx, nil
}

func SendTransactionToArc(tx *bt.Tx) error {
	client := &http.Client{}

	req, err := http.NewRequest("POST", "https://api.taal.com/arc/v1/tx", bytes.NewBuffer(tx.ExtendedBytes()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	message := string(body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Request failed: %s (%d)\n", message, resp.StatusCode)
	}

	fmt.Printf("Status: %d, %s\n", resp.StatusCode, message)

	return nil
}
