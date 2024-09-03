package integrationtest

import (
	"database/sql"
	"encoding/hex"
	"testing"

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

func pruneTables(db *sql.DB) error {
	_, err := db.Exec("TRUNCATE TABLE metamorph.transactions;")
	if err != nil {
		return err
	}

	return nil
}
