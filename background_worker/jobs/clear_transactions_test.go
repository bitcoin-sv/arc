package jobs

import (
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/store"
	. "github.com/bitcoin-sv/arc/database_testing"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ClearTransactionsSuite struct {
	DatabaseTestSuite
}

func (s *ClearTransactionsSuite) Test() {
	params := ClearRecrodsParams{
		DBConnectionParams:  DefaultParams,
		RecordRetentionDays: 10,
	}

	// Add "fresh" blocks
	for i := 0; i < 5; i++ {
		tx := GetTestTransaction()
		tx.InsertedAt = time.Now()
		s.InsertTransaction(tx)
	}

	// Add "expired" blocks
	for i := 0; i < 5; i++ {
		tx := GetTestTransaction()
		tx.InsertedAt = time.Now().Add(-20 * 24 * time.Hour)
		s.InsertTransaction(tx)
	}

	db, err := sqlx.Open("postgres", params.String())
	require.NoError(s.T(), err)

	err = ClearTransactions(params)
	require.NoError(s.T(), err)

	var txs []store.Transaction
	require.NoError(s.T(), db.Select(&txs, "SELECT * FROM transactions"))

	assert.Len(s.T(), txs, 5)
}

func TestRunClearTransactionsSuite(t *testing.T) {
	s := new(ClearTransactionsSuite)
	suite.Run(t, s)

}
