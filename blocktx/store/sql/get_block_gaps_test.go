package sql

import (
	"context"
	"database/sql"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"testing"

	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GetBlockGapTestSuite struct {
	BlockTXDBTestSuite
}

func (s *GetBlockGapTestSuite) Test() {
	db, err := sql.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	st := &SQL{db: db}

	fixtures, err := testfixtures.New(
		testfixtures.Database(db),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures/get_block_gaps"), // The directory containing the YAML files
	)
	require.NoError(s.T(), err)

	err = fixtures.Load()
	require.NoError(s.T(), err)
	ctx := context.Background()

	blockGaps, err := st.GetBlockGaps(ctx, 7)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, len(blockGaps))

	hash822014, err := chainhash.NewHashFromStr("0000000000000000025855b62f4c2e3732dad363a6f2ead94e4657ef96877067")
	require.NoError(s.T(), err)
	hash822019, err := chainhash.NewHashFromStr("00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656")
	require.NoError(s.T(), err)

	expectedBlockGaps := []*store.BlockGap{
		{
			Height: 8220119,
			Hash:   hash822019,
		},
		{
			Height: 822014,
			Hash:   hash822014,
		},
	}

	require.ElementsMatch(s.T(), expectedBlockGaps, blockGaps)
}

func TestGetBlockGapTestSuite(t *testing.T) {
	s := new(GetBlockGapTestSuite)
	suite.Run(t, s)
}
