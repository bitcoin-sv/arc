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

type ClearBlockTransactionsMapSuite struct {
	DatabaseTestSuite
}

func (s *ClearBlockTransactionsMapSuite) Test() {
	params := ClearRecrodsParams{
		DBConnectionParams:  DefaultParams,
		RecordRetentionDays: 10,
	}

	// Add "fresh" blocks
	for i := 0; i < 5; i++ {
		m := store.BlockTransactionMap{
			BlockID:       int64(i),
			TransactionID: int64(i),
			Pos:           int64(i),
		}
		m.InsertedAt = time.Now()
		s.InsertBlockTransactionMap(&m)
	}

	// Add "expired" blocks
	for i := 0; i < 5; i++ {
		m := store.BlockTransactionMap{
			BlockID:       10 + int64(i),
			TransactionID: 10 + int64(i),
			Pos:           10 + int64(i),
		}
		m.InsertedAt = time.Now().Add(-20 * 24 * time.Hour)
		s.InsertBlockTransactionMap(&m)
	}

	db, err := sqlx.Open("postgres", params.String())
	require.NoError(s.T(), err)

	err = ClearBlockTransactionsMap(params)
	require.NoError(s.T(), err)

	var mps []store.BlockTransactionMap
	require.NoError(s.T(), db.Select(&mps, "SELECT * FROM block_transactions_map"))

	assert.Len(s.T(), mps, 5)
}

func TestRunCClearBlockTransactionsMapSuite(t *testing.T) {
	s := new(ClearBlockTransactionsMapSuite)
	suite.Run(t, s)

	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
