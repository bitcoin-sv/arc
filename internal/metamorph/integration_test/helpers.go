package integrationtest

import (
	"database/sql"
	"encoding/hex"
	"testing"

	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
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

func pruneTables(t *testing.T, db *sql.DB) {
	testutils.PruneTables(t, db, "metamorph.transactions")
}
