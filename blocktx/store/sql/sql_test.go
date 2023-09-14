package sql

import (
	"bytes"
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"

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

	firstHash, err := chainhash.NewHashFromStr("b042f298deabcebbf15355aa3a13c7d7cfe96c44ac4f492735f936f8e50d06f6")
	require.NoError(t, err)

	transactions := []*blocktx_api.TransactionAndSource{
		{Hash: firstHash[:], Source: "TEST"},
		{Hash: []byte("test transaction hash 2"), Source: "TEST"},
		{Hash: []byte("test transaction hash 3"), Source: "TEST"},
		{Hash: []byte("test transaction hash 4"), Source: "TEST"},
		{Hash: []byte("test transaction hash 5"), Source: "TEST"},
	}

	for _, txn := range transactions {
		_, _, _, _, err = s.RegisterTransaction(ctx, txn)
		require.NoError(t, err)
	}

	_, _, _, _, err = s.RegisterTransaction(ctx, &blocktx_api.TransactionAndSource{
		Hash:   firstHash[:],
		Source: "TEST",
	})
	require.NoError(t, err)

	source, err := s.GetTransactionSource(ctx, firstHash)
	require.NoError(t, err)
	assert.Equal(t, "TEST", source)

	err = s.InsertBlockTransactions(ctx, blockId, transactions, make([]string, len(transactions), len(transactions)))
	require.NoError(t, err)

	txns, err := s.GetBlockTransactions(ctx, block)
	require.NoError(t, err)

	for i, txn := range txns.Transactions {
		assert.Equal(t, bytes.Equal(transactions[i].Hash, txn.Hash), true)
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
