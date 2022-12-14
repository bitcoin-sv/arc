package sql

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	pb "github.com/TAAL-GmbH/arcblocktx_api"

	"github.com/ordishs/gocore"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	dbHost, _     = gocore.Config().Get("dbHost", "localhost")
	dbPort, _     = gocore.Config().GetInt("dbPort", 5432)
	dbName, _     = gocore.Config().Get("dbName", "blocktx")
	dbUser, _     = gocore.Config().Get("dbUser", "blocktx")
	dbPassword, _ = gocore.Config().Get("dbPassword", "blocktx")
	dbInfo        = fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%d", dbUser, dbPassword, dbName, dbHost, dbPort)
)

func TestInOut(t *testing.T) {
	ctx := context.Background()

	block := &pb.Block{
		Hash:   []byte("test block hash"),
		Header: []byte("test block header"),
		Height: 1,
	}

	s, err := New("postgres", dbHost, dbUser, dbPassword, dbName, dbPort)
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

	s, err := New("postgres", dbHost, dbUser, dbPassword, dbName, dbPort)
	require.NoError(t, err)

	height := uint64(1000000)

	b, err := s.GetBlockForHeight(ctx, height)
	require.Error(t, err)
	assert.Nil(t, b)
}
