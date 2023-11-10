package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	. "github.com/bitcoin-sv/arc/database_testing"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SQLTest struct {
	DatabaseTestSuite
}

func (s *SQLTest) Test() {
	ctx := context.Background()

	block := &blocktx_api.Block{
		Hash:         []byte("test block hash"),
		MerkleRoot:   []byte("test merkleroot"),
		PreviousHash: []byte("test prevhash"),
		Height:       1,
	}

	store, err := NewPostgresStore(DefaultParams)
	require.NoError(s.T(), err)

	blockId, err := store.InsertBlock(ctx, block)
	require.NoError(s.T(), err)

	firstHash, err := chainhash.NewHash([]byte(GetRandomBytes()))
	require.NoError(s.T(), err)

	transactions := []*blocktx_api.TransactionAndSource{
		{Hash: firstHash[:], Source: "TEST"},
		{Hash: []byte(GetRandomBytes()), Source: "TEST"},
		{Hash: []byte(GetRandomBytes()), Source: "TEST"},
		{Hash: []byte(GetRandomBytes()), Source: "TEST"},
		{Hash: []byte(GetRandomBytes()), Source: "TEST"},
	}

	for _, txn := range transactions {
		_, _, _, _, err = store.RegisterTransaction(ctx, txn)
		require.NoError(s.T(), err)
	}

	_, _, _, _, err = store.RegisterTransaction(ctx, &blocktx_api.TransactionAndSource{
		Hash:   firstHash[:],
		Source: "TEST",
	})
	require.NoError(s.T(), err)

	source, err := store.GetTransactionSource(ctx, firstHash)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "TEST", source)

	err = store.InsertBlockTransactions(ctx, blockId, transactions, make([]string, len(transactions)))
	require.NoError(s.T(), err)

	txns, err := store.GetBlockTransactions(ctx, block)
	require.NoError(s.T(), err)

	for i, txn := range txns.Transactions {
		assert.Equal(s.T(), transactions[i].Hash, txn.Hash)
	}

	height := uint64(1)

	err = store.SetOrphanHeight(ctx, height, false)
	require.NoError(s.T(), err)

	block2, err := store.GetBlockForHeight(ctx, height)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), string(block.Hash), string(block2.Hash))

	err = store.OrphanHeight(ctx, height)
	require.NoError(s.T(), err)

	block3, err := store.GetBlockForHeight(ctx, height)
	require.Error(s.T(), err)
	assert.Nil(s.T(), block3)

}

func TestSQLTest(t *testing.T) {
	s := new(SQLTest)
	suite.Run(t, s)
}
