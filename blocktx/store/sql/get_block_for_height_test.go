package sql

import (
	"context"
	"testing"

	. "github.com/bitcoin-sv/arc/database_testing"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GetBlockByHeightTestSuite struct {
	DatabaseTestSuite
}

func (s *GetBlockByHeightTestSuite) TestGetBlockByHeight() {
	block := GetTestBlock()

	s.InsertBlock(block)
	store, err := NewPostgresStore(DefaultParams)
	require.NoError(s.T(), err)

	b, err := store.GetBlockForHeight(context.Background(), uint64(block.Height))

	require.NoError(s.T(), err)
	assert.Equal(s.T(), block.Hash, string(b.Hash))
}

func TestGetBlockByHeightSuite(t *testing.T) {
	s := new(GetBlockByHeightTestSuite)
	suite.Run(t, s)

	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
