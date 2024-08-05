package integrationtest

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"testing"

	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
)

func revChainhash(t *testing.T, hashString string) *chainhash.Hash {
	hash, err := hex.DecodeString(hashString)
	require.NoError(t, err)
	txHash, err := chainhash.NewHash(hash)
	require.NoError(t, err)

	return txHash
}

func loadFixtures(db *sql.DB, path string) error {
	fixtures, err := testfixtures.New(
		testfixtures.Database(db),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory(path), // The directory containing the YAML files
	)
	if err != nil {
		log.Fatalf("failed to create fixtures: %v", err)
	}

	err = fixtures.Load()
	if err != nil {
		return fmt.Errorf("failed to load fixtures: %v", err)
	}

	return nil
}

func pruneTables(db *sql.DB) error {
	_, err := db.Exec("TRUNCATE TABLE metamorph.transactions;")
	if err != nil {
		return err
	}

	return nil
}
