package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/store"
	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GetTransactionMerklePathSuite struct {
	BlockTXDBTestSuite
}

func (s *GetTransactionMerklePathSuite) Test() {
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

	h, err := chainhash.NewHash([]byte(tx.Hash))
	require.NoError(s.T(), err)

	path, err := store.GetTransactionMerklePath(context.Background(), h)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), tx.MerklePath, path)
}

func TestGetTransactionMerklePathSuite(t *testing.T) {
	s := new(GetTransactionMerklePathSuite)
	suite.Run(t, s)
}
