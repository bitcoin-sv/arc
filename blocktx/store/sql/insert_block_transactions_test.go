package sql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	. "github.com/bitcoin-sv/arc/database_testing"
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

func (s *InsertBlockTransactionsSuite) Test() {
	s.T().Skip("Fails due to postgres constraint error")
	pstore, err := NewPostgresStore(DefaultParams)
	require.NoError(s.T(), err)

	tx := blocktx_api.TransactionAndSource{
		Hash:   []byte(GetRandomBytes()),
		Source: GetRandomBytes(),
	}
	testMerkle := "testpath"
	testBlockID := uint64(10)
	err = pstore.InsertBlockTransactions(context.Background(), testBlockID, []*blocktx_api.TransactionAndSource{&tx}, []string{testMerkle})
	require.NoError(s.T(), err)

	d, err := sqlx.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	var storedtx Tx

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(tx.GetHash()))
	require.NoError(s.T(), err)

	assert.Equal(s.T(), string(tx.GetHash()), storedtx.Hash)
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
}
