package integrationtest

import (
	"database/sql"
	"testing"

	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

func pruneTables(t *testing.T, db *sql.DB) {
	testutils.PruneTables(t, db, "metamorph.transactions")
}
