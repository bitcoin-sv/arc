package sql

import (
	"bytes"
	"context"
	"testing"

	pb "github.com/TAAL-GmbH/arc/blocktx_api"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInOut(t *testing.T) {
	ctx := context.Background()

	block := &pb.Block{
		Hash:   []byte("test block hash"),
		Header: []byte("test block header"),
		Height: 1,
	}

	s, err := NewSQLStore("sqlite")
	require.NoError(t, err)

	blockId, err := s.InsertBlock(ctx, block)
	require.NoError(t, err)

	transactions := []*pb.Transaction{
		{Hash: []byte("test transaction hash 1")},
		{Hash: []byte("test transaction hash 2")},
		{Hash: []byte("test transaction hash 3")},
		{Hash: []byte("test transaction hash 4")},
		{Hash: []byte("test transaction hash 5")},
	}

	err = s.InsertBlockTransactions(ctx, blockId, transactions)
	require.NoError(t, err)

	txns, err := s.GetBlockTransactions(ctx, block)
	require.NoError(t, err)

	for i, txn := range txns.Transactions {
		assert.Equal(t, bytes.Equal(transactions[i].Hash, txn.Hash), true)
	}

	blocks, err := s.GetTransactionBlocks(ctx, transactions[0])
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

	s, err := NewSQLStore("sqlite")
	require.NoError(t, err)

	height := uint64(1000000)

	b, err := s.GetBlockForHeight(ctx, height)
	require.Error(t, err)
	assert.Nil(t, b)
}
