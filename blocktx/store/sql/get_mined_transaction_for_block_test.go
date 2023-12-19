package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MinedTransactionForBlockSuite struct {
	BlockTXDBTestSuite
}

func (s *MinedTransactionForBlockSuite) Test() {
	block := GetTestBlock()
	tx := GetTestTransaction()
	s.InsertBlock(block)

	s.InsertTransaction(tx)

	s.InsertBlockTransactionMap(&store.BlockTransactionMap{
		BlockID:       block.ID,
		TransactionID: int64(tx.ID),
		Pos:           2,
	})

	store, err := NewPostgresStore(DefaultParams)
	require.NoError(s.T(), err)
	bs := &blocktx_api.BlockAndSource{
		Hash:   []byte(block.Hash),
		Source: tx.Source,
	}
	require.NoError(s.T(), err)
	b, err := store.GetMinedTransactionsForBlock(context.Background(), bs)

	require.NoError(s.T(), err)

	assert.Equal(s.T(), block.Hash, string(b.GetBlock().GetHash()))
	assert.Equal(s.T(), tx.Hash, string(b.GetTransactions()[0].GetHash()))
}

func TestMinedTransactionForBlock(t *testing.T) {
	s := new(MinedTransactionForBlockSuite)
	suite.Run(t, s)
}
