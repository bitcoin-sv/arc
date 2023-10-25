package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MinedTransactionForBlockSuite struct {
	DatabaseTestSuite
}

func (s MinedTransactionForBlockSuite) Run() {
	block := GetTestBlock()
	tx := GetTestTransaction()
	s.InsertBlock(block)

	s.InsertTransaction(tx)

	s.InsertBlockTransactionMap(&store.BlockTransactionMap{
		BlockID:       block.ID,
		TransactionID: tx.ID,
		Pos:           2,
	})

	store, err := NewPostgresStore(defaultParams)
	require.NoError(s.T(), err)
	bs := &blocktx_api.BlockAndSource{
		Hash:   []byte(block.Hash),
		Source: tx.Source,
	}
	require.NoError(s.T(), err)
	b, err := store.GetMinedTransactionsForBlock(context.Background(), bs)

	require.NoError(s.T(), err)

	assert.Equal(s.T(), block.Hash, string(b.Block.Hash))
	assert.Equal(s.T(), tx.Hash, string(b.Transactions[0].Hash))
}

func TestMinedTransactionForBlock(t *testing.T) {
	s := new(MinedTransactionForBlockSuite)
	suite.Run(t, s)
	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
