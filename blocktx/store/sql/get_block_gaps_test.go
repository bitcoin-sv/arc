package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/store"
	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/jmoiron/sqlx"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GetBlockGapTestSuite struct {
	BlockTXDBTestSuite
}

func (s *GetBlockGapTestSuite) Test() {
	db, err := sqlx.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	st := &SQL{db: db}

	fixtures, err := testfixtures.New(
		testfixtures.Database(db.DB),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures/get_block_gaps"), // The directory containing the YAML files
	)
	require.NoError(s.T(), err)

	err = fixtures.Load()
	require.NoError(s.T(), err)
	ctx := context.Background()

	blockGaps, err := st.GetBlockGaps(ctx, 20)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, len(blockGaps))

	hash822014, err := chainhash.NewHashFromStr("0000000000000000025855b62f4c2e3732dad363a6f2ead94e4657ef96877067")
	require.NoError(s.T(), err)
	hash822019, err := chainhash.NewHashFromStr("00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656")
	require.NoError(s.T(), err)
	hash822011, err := chainhash.NewHashFromStr("000000000000000002a0926c51854d2bd525c26026ab0f178ca07f723b31033a")
	require.NoError(s.T(), err)

	expectedBlockGaps := []*store.BlockGap{
		{
			Height: 822019,
			Hash:   hash822019,
		},
		{
			Height: 822014,
			Hash:   hash822014,
		},
		{
			Height: 822011,
			Hash:   hash822011,
		},
	}

	require.ElementsMatch(s.T(), expectedBlockGaps, blockGaps)
}

func TestGetBlockGapTestSuite(t *testing.T) {
	s := new(GetBlockGapTestSuite)
	suite.Run(t, s)
}
