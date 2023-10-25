package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type InsertBlockTransactionsSuite struct {
	DatabaseTestSuite
}

type Tx struct {
	ID         int64  `db:"id"`
	Hash       string `db:"hash"`
	MerklePath string `db:"merkle_path"`
}

func (s InsertBlockTransactionsSuite) Run() {

	pstore, err := NewPostgresStore(defaultParams)
	require.NoError(s.T(), err)

	tx := blocktx_api.TransactionAndSource{
		Hash:   []byte(getRandomBytes()),
		Source: getRandomBytes(),
	}
	testMerkle := "testpath"
	testBlockID := uint64(10)
	err = pstore.InsertBlockTransactions(context.Background(), testBlockID, []*blocktx_api.TransactionAndSource{&tx}, []string{testMerkle})
	require.NoError(s.T(), err)

	d, err := sqlx.Open("postgres", defaultParams.String())
	require.NoError(s.T(), err)

	var storedtx Tx

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(tx.Hash))
	require.NoError(s.T(), err)

	assert.Equal(s.T(), string(tx.Hash), storedtx.Hash)
	assert.Equal(s.T(), testMerkle, storedtx.MerklePath)

	var mp store.BlockTransactionMap
	err = d.Get(&mp, "SELECT blockid, txid, pos from block_transactions_map WHERE txid=$1", storedtx.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), storedtx.ID, mp.TransactionID)
	assert.Equal(s.T(), testBlockID, uint64(mp.BlockID))
}

func TestInsertBlockTransactionsSuite(t *testing.T) {
	s := new(InsertBlockTransactionsSuite)
	suite.Run(t, s)
	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
