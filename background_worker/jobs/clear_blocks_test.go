package jobs

import (
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/store"
	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/go-testfixtures/testfixtures/v3"
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
	params := ClearRecrodsParams{
		DBConnectionParams:  DefaultParams,
		RecordRetentionDays: 10,
	}

	db, err := sqlx.Open("postgres", params.String())
	require.NoError(s.T(), err)

	fixtures, err := testfixtures.New(
		testfixtures.Database(db.DB),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures"), // The directory containing the YAML files
	)
	require.NoError(s.T(), err)

	err = fixtures.Load()
	require.NoError(s.T(), err)

	now := time.Date(2023, 12, 22, 12, 0, 0, 0, time.UTC)

	clearJob := NewClearJob(WithNow(func() time.Time {
		return now
	}))

	err = clearJob.ClearBlocks(params)
	require.NoError(s.T(), err)

	var blocks []store.Block

	require.NoError(s.T(), db.Select(&blocks, "SELECT id FROM blocks"))

	assert.Len(s.T(), blocks, 1)
}

func TestRunClear(t *testing.T) {
	s := new(ClearJobSuite)
	suite.Run(t, s)
}
