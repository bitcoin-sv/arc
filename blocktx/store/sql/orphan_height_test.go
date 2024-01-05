package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/store"
	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OrphanHeightSutie struct {
	BlockTXDBTestSuite
}

func (s *OrphanHeightSutie) Test() {
	block := GetTestBlock()

	s.InsertBlock(block)

	pstore, err := NewPostgresStore(DefaultParams)
	require.NoError(s.T(), err)

	err = pstore.OrphanHeight(context.Background(), uint64(block.Height))
	require.NoError(s.T(), err)

	d, err := sqlx.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	var storedblock store.Block
	err = d.Get(&storedblock, "SELECT id, hash, merkle_path,orphanedyn from blocks WHERE height=$1", block.Height)
	require.NoError(s.T(), err)

	assert.True(s.T(), storedblock.Orphaned)
}

func TestOrphanHeightSutie(t *testing.T) {
	s := new(OrphanHeightSutie)
	suite.Run(t, s)
}
