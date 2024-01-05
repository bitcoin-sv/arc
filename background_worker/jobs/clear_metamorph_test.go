package jobs

import (
	"testing"
	"time"

	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ClearMetamorphSuite struct {
	MetamorphDBTestSuite
}

func (s *ClearMetamorphSuite) Test() {

	for i := 0; i < 5; i++ {
		blk := GetTestMMBlock()
		blk.InsertedAt = time.Now().Add(-20 * 24 * time.Hour)
		s.InsertBlock(blk)
	}

	for i := 0; i < 5; i++ {
		blk := GetTestMMBlock()
		blk.InsertedAt = time.Now().Add(-1 * 24 * time.Hour)
		s.InsertBlock(blk)
	}

	for i := 0; i < 5; i++ {
		tx := GetTestMMTransaction()
		tx.InsertedAt = time.Now().Add(-20 * 24 * time.Hour)

		s.InsertTransaction(tx)
	}

	for i := 0; i < 5; i++ {
		tx := GetTestMMTransaction()
		tx.InsertedAt = time.Now().Add(-1 * 24 * time.Hour)

		s.InsertTransaction(tx)
	}

	err := ClearMetamorph(ClearRecordsParams{
		DBConnectionParams:  DefaultMMParams,
		RecordRetentionDays: 14,
	})

	require.NoError(s.T(), err)

	db, err := sqlx.Open("postgres", DefaultMMParams.String())
	require.NoError(s.T(), err)

	var blks []store.Block
	require.NoError(s.T(), db.Select(&blks, "SELECT * from metamorph.blocks"))

	assert.Len(s.T(), blks, 5)

	var stx []store.Transaction
	require.NoError(s.T(), db.Select(&stx, "SELECT * from metamorph.transactions"))
	assert.Len(s.T(), stx, 5)

}

func TestRunClearMM(t *testing.T) {
	s := new(ClearMetamorphSuite)
	suite.Run(t, s)
}
