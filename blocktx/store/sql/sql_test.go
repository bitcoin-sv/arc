package sql

import (
	"bytes"
	"context"
	"testing"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInOut(t *testing.T) {
	ctx := context.Background()

	block := &blocktx_api.Block{
		Hash:         []byte("test block hash"),
		MerkleRoot:   []byte("test merkleroot"),
		PreviousHash: []byte("test prevhash"),
		Height:       1,
	}

	s, err := New("sqlite_memory")
	require.NoError(t, err)

	blockId, err := s.InsertBlock(ctx, block)
	require.NoError(t, err)

	firstHash := []byte("test transaction hash 1")

	transactions := []*blocktx_api.TransactionAndSource{
		{Hash: firstHash},
		{Hash: []byte("test transaction hash 2")},
		{Hash: []byte("test transaction hash 3")},
		{Hash: []byte("test transaction hash 4")},
		{Hash: []byte("test transaction hash 5")},
	}

	for _, txn := range transactions {
		err = s.InsertTransaction(ctx, txn)
		require.NoError(t, err)
	}

	err = s.InsertTransaction(ctx, &blocktx_api.TransactionAndSource{
		Hash:   firstHash,
		Source: "TEST",
	})
	require.NoError(t, err)

	source, err := s.GetTransactionSource(ctx, firstHash)
	require.NoError(t, err)
	assert.Equal(t, "TEST", source)

	err = s.InsertBlockTransactions(ctx, blockId, transactions)
	require.NoError(t, err)

	txns, err := s.GetBlockTransactions(ctx, block)
	require.NoError(t, err)

	for i, txn := range txns.Transactions {
		assert.Equal(t, bytes.Equal(transactions[i].Hash, txn.Hash), true)
	}

	blocks, err := s.GetTransactionBlocks(ctx, &blocktx_api.Transaction{
		Hash: transactions[0].Hash,
	})
	require.NoError(t, err)

	for i, block := range blocks.Blocks {
		assert.Equal(t, bytes.Equal(blocks.Blocks[i].Hash, block.Hash), true)
	}

	height := uint64(1)

	err = s.SetOrphanHeight(ctx, height, false)
	require.NoError(t, err)

	block2, err := s.GetBlockForHeight(ctx, height)
	require.NoError(t, err)
	assert.Equal(t, bytes.Equal(block.Hash, block2.Hash), true)

	err = s.OrphanHeight(ctx, height)
	require.NoError(t, err)

	block3, err := s.GetBlockForHeight(ctx, height)
	require.Error(t, err)
	assert.Nil(t, block3)

}

func TestBlockNotExists(t *testing.T) {
	ctx := context.Background()

	s, err := New("sqlite_memory")
	require.NoError(t, err)

	height := uint64(1000000)

	b, err := s.GetBlockForHeight(ctx, height)
	require.Error(t, err)
	assert.Nil(t, b)
}
