package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/store"
	. "github.com/bitcoin-sv/arc/database_testing"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GetLastProcessedBlockSuite struct {
	DatabaseTestSuite
}

func (s *GetBlockByHeightTestSuite) TestGetLastProcessedBlock() {
	block := GetTestBlock()
	tx := GetTestTransaction()
	s.InsertBlock(block)

	s.InsertTransaction(tx)

	s.InsertBlockTransactionMap(&store.BlockTransactionMap{
		BlockID:       block.ID,
		TransactionID: tx.ID,
		Pos:           2,
	})

	st, err := NewPostgresStore(DefaultParams)
	require.NoError(s.T(), err)

	blk, err := st.GetLastProcessedBlock(context.Background())
	require.NoError(s.T(), err)
	assert.Equal(s.T(), block.Hash, string(blk.Hash))
}

func TestGetLastProcessedBlock(t *testing.T) {
	s := new(GetLastProcessedBlockSuite)
	suite.Run(t, s)
	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
