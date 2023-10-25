package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OrphanHeightSutie struct {
	DatabaseTestSuite
}

func (s *OrphanHeightSutie) Test() {
	block := GetTestBlock()

	s.InsertBlock(block)

	pstore, err := NewPostgresStore(defaultParams)
	require.NoError(s.T(), err)

	err = pstore.OrphanHeight(context.Background(), 10)
	require.NoError(s.T(), err)

	d, err := sqlx.Open("postgres", defaultParams.String())
	require.NoError(s.T(), err)

	var storedblock store.Block
	err = d.Get(&storedblock, "SELECT id, hash, merkle_path,orphanedyn from blocks WHERE hash=$1", block.Hash)
	require.NoError(s.T(), err)

	assert.True(s.T(), block.Orphaned)
}

func TestOrphanHeightSutie(t *testing.T) {
	s := new(OrphanHeightSutie)
	suite.Run(t, s)
	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
