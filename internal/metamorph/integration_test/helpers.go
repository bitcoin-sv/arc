package integrationtest

import (
	"database/sql"
	"testing"

	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
)

func pruneTables(t *testing.T, db *sql.DB) {
	testutils.PruneTables(t, db, "metamorph.transactions")
}
