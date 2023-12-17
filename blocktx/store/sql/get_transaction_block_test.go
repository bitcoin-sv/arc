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

type GetTransactionBlockSuite struct {
	BlockTXDBTestSuite
}

func (s *GetTransactionBlockSuite) Test() {
	block := GetTestBlock()
	tx := GetTestTransaction()
	s.InsertBlock(block)

	s.InsertTransaction(tx)

	s.InsertBlockTransactionMap(&store.BlockTransactionMap{
		BlockID:       block.ID,
		TransactionID: tx.ID,
		Pos:           2,
	})

	store, err := NewPostgresStore(DefaultParams)
	require.NoError(s.T(), err)

	b, err := store.GetTransactionBlock(context.Background(), &blocktx_api.Transaction{
		Hash: []byte(tx.Hash),
	})
	require.NoError(s.T(), err)

	assert.Equal(s.T(), block.Hash, string(b.Hash))
}

func TestGetTransactionBlock(t *testing.T) {
	s := new(GetTransactionBlockSuite)
	suite.Run(t, s)
}
