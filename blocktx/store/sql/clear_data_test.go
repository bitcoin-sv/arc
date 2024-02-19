package sql

import (
	"context"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/database_testing"
	"github.com/go-testfixtures/testfixtures/v3"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type ClearJobSuite struct {
	database_testing.BlockTXDBTestSuite
}

func (s *ClearJobSuite) Test() {
	db, err := sqlx.Open("postgres", database_testing.DefaultParams.String())
	require.NoError(s.T(), err)

	st := &SQL{
		db:     db,
		engine: postgresEngine,
		now: func() time.Time {
			return time.Date(2023, 12, 22, 12, 0, 0, 0, time.UTC)
		},
	}

	fixtures, err := testfixtures.New(
		testfixtures.Database(db.DB),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures/clear_data"), // The directory containing the YAML files
	)
	require.NoError(s.T(), err)

	err = fixtures.Load()
	require.NoError(s.T(), err)

	resp, err := st.ClearBlocktxTable(context.Background(), 10, "blocks")
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(1), resp.Rows)

	var blocks []store.Block

	require.NoError(s.T(), db.Select(&blocks, "SELECT id FROM blocks"))

	require.Len(s.T(), blocks, 1)

	resp, err = st.ClearBlocktxTable(context.Background(), 10, "block_transactions_map")
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(5), resp.Rows)

	var mps []store.BlockTransactionMap
	require.NoError(s.T(), db.Select(&mps, "SELECT blockid FROM block_transactions_map"))

	require.Len(s.T(), mps, 5)

	resp, err = st.ClearBlocktxTable(context.Background(), 10, "transactions")
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(5), resp.Rows)

	var txs []store.Transaction
	require.NoError(s.T(), db.Select(&txs, "SELECT id FROM transactions"))

	require.Len(s.T(), txs, 5)
}

func TestRunClear(t *testing.T) {
	s := new(ClearJobSuite)
	suite.Run(t, s)
}
