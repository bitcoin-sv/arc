package sql

import (
	"context"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"testing"

	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/go-testfixtures/testfixtures/v3"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SetBlockProcessingTestSuite struct {
	BlockTXDBTestSuite
}

func (s *SetBlockProcessingTestSuite) Test() {
	db, err := sqlx.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	st := &SQL{db: db}

	fixtures, err := testfixtures.New(
		testfixtures.Database(db.DB),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures/block_processing"), // The directory containing the YAML files
	)
	require.NoError(s.T(), err)

	err = fixtures.Load()
	require.NoError(s.T(), err)
	ctx := context.Background()

	bh1, err := chainhash.NewHashFromStr("0000000000000000021957ec4c210f9b6327cfe1ad77a29aba39667ecf687474")
	require.NoError(s.T(), err)

	processedBy, err := st.SetBlockProcessing(ctx, bh1, "pod-1")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "pod-1", processedBy)

	// set a second time, expect error
	processedBy, err = st.SetBlockProcessing(ctx, bh1, "pod-1")
	require.ErrorIs(s.T(), err, store.ErrBlockProcessingDuplicateKey)
	require.Equal(s.T(), "pod-1", processedBy)

	bhInProgress, err := chainhash.NewHashFromStr("000000000000000005aa39a25e7e8bf440c270ec9a1bd30e99ab026f39207ef9")
	require.NoError(s.T(), err)

	blockHashes, err := st.GetBlockHashesProcessingInProgress(ctx, "pod-2")
	require.NoError(s.T(), err)
	require.Len(s.T(), blockHashes, 1)
	require.True(s.T(), bhInProgress.IsEqual(blockHashes[0]))

	err = st.DelBlockProcessing(ctx, bhInProgress, "pod-1")
	require.ErrorIs(s.T(), err, store.ErrBlockNotFound)

	err = st.DelBlockProcessing(ctx, bhInProgress, "pod-2")
	require.NoError(s.T(), err)

	blockHashes, err = st.GetBlockHashesProcessingInProgress(ctx, "pod-2")
	require.NoError(s.T(), err)
	require.Len(s.T(), blockHashes, 0)

}

func TestSetBlockProcessingTestSuite(t *testing.T) {
	s := new(SetBlockProcessingTestSuite)
	suite.Run(t, s)
}
