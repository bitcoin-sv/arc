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

type ClearJobSuite struct {
	DatabaseTestSuite
}

func (s *ClearJobSuite) Test() {
	params := Params{
		DBConnectionParams: DefaultParams,
		OlderThan:          10,
	}

	// Add "fresh" blocks
	for i := 0; i < 5; i++ {
		block := GetTestBlock()
		block.InsertedAt = time.Now()
		s.InsertBlock(block)
	}

	// Add "expired" blocks
	for i := 0; i < 5; i++ {
		block := GetTestBlock()
		block.InsertedAt = time.Now().Add(-20 * 24 * time.Hour)
		s.InsertBlock(block)
	}

	db, err := sqlx.Open("postgres", params.String())
	require.NoError(s.T(), err)

	err = ClearBlocks(params)
	require.NoError(s.T(), err)

	var blocks []store.Block
	require.NoError(s.T(), db.Select(&blocks, "SELECT * FROM blocks"))

	assert.Len(s.T(), blocks, 5)
}

func TestRunClear(t *testing.T) {
	s := new(ClearJobSuite)
	suite.Run(t, s)

	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
