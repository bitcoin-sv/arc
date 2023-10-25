package sql

import (
	"context"
	"testing"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GetBlockTestSuite struct {
	DatabaseTestSuite
}

func (s *GetBlockTestSuite) Test() {
	block := GetTestBlock()

	s.InsertBlock(block)
	store, err := NewPostgresStore(defaultParams)
	require.NoError(s.T(), err)

	h, err := chainhash.NewHash([]byte(block.Hash))
	require.NoError(s.T(), err)
	b, err := store.GetBlock(context.Background(), h)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), block.Hash, string(b.Hash))
}

func TestGetBlockTestSuite(t *testing.T) {
	s := new(GetBlockTestSuite)
	suite.Run(t, s)
	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
