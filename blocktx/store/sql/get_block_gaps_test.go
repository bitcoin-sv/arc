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

	blockGaps, err := st.GetBlockGaps(ctx, 12)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 5, len(blockGaps))

	hash822014, err := chainhash.NewHashFromStr("0000000000000000025855b62f4c2e3732dad363a6f2ead94e4657ef96877067")
	require.NoError(s.T(), err)
	hash822019, err := chainhash.NewHashFromStr("00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656")
	require.NoError(s.T(), err)
	hash822011, err := chainhash.NewHashFromStr("000000000000000002a0926c51854d2bd525c26026ab0f178ca07f723b31033a")
	require.NoError(s.T(), err)
	hash822020, err := chainhash.NewHashFromStr("00000000000000000a5c4d27edc0178e953a5bb0ab0081e66cb30c8890484076")
	require.NoError(s.T(), err)
	hash822009, err := chainhash.NewHashFromStr("00000000000000000e3f79a11df0f07581b91bc7a8c7d80e9a1264a4b173d74a")
	require.NoError(s.T(), err)

	expectedBlockGaps := []*store.BlockGap{
		{ // gap
			Height: 822019,
			Hash:   hash822019,
		},
		{ // gap
			Height: 822014,
			Hash:   hash822014,
		},
		{ // gap
			Height: 822011, // gap
			Hash:   hash822011,
		},
		{ // processing not finished
			Height: 822020,
			Hash:   hash822020,
		},
		{ // gap
			Height: 822009,
			Hash:   hash822009,
		},
	}

	require.ElementsMatch(s.T(), expectedBlockGaps, blockGaps)
}

func TestGetBlockGapTestSuite(t *testing.T) {
	s := new(GetBlockGapTestSuite)
	suite.Run(t, s)
}
