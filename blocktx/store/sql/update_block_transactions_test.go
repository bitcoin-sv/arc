package sql

import (
	"context"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/database_testing"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UpdateBlockTransactionsSuite struct {
	database_testing.BlockTXDBTestSuite
}

type Tx struct {
	ID         int64  `db:"id"`
	Hash       []byte `db:"hash"`
	MerklePath string `db:"merkle_path"`
}

func (s *UpdateBlockTransactionsSuite) Test() {
	db, err := sqlx.Open("postgres", database_testing.DefaultParams.String())
	require.NoError(s.T(), err)

	st := &SQL{
		db:     db,
		engine: postgresEngine,
	}

	fixtures, err := testfixtures.New(
		testfixtures.Database(db.DB),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures/insert_block_transactions"),
	)
	require.NoError(s.T(), err)

	err = fixtures.Load()
	require.NoError(s.T(), err)
	testMerklePaths := []string{"test1", "test2", "test3"}
	testBlockID := uint64(9736)

	txHash1, err := chainhash.NewHashFromStr("3b8c1676470f44043069f66fdc0d5df9bdad1865a8f03b8da1268359802b7376")
	require.NoError(s.T(), err)

	txHash2, err := chainhash.NewHashFromStr("b0926372c449731ffb84b7d2f808087d4b5e6e26aafee872232bbcd5a5854e16")
	require.NoError(s.T(), err)

	txHashNotRegistered, err := chainhash.NewHashFromStr("edd33fdcdfa68444d227780e2b62a4437c00120c5320d2026aeb24a781f4c3f1")
	require.NoError(s.T(), err)

	_, err = st.UpdateBlockTransactions(context.Background(), testBlockID, []*blocktx_api.TransactionAndSource{
		{
			Hash: txHash1[:],
		},
		{
			Hash: txHash2[:],
		},
		{
			Hash: txHashNotRegistered[:],
		},
	}, []string{"test1"})

	require.ErrorContains(s.T(), err, "transactions (len=3) and Merkle paths (len=1) have not the same lengths")

	_, err = st.UpdateBlockTransactions(context.Background(), testBlockID, []*blocktx_api.TransactionAndSource{
		{
			Hash: txHash1[:],
		},
		{
			Hash: txHash2[:],
		},
		{
			Hash: txHashNotRegistered[:],
		},
	}, testMerklePaths)
	require.NoError(s.T(), err)

	d, err := sqlx.Open("postgres", database_testing.DefaultParams.String())
	require.NoError(s.T(), err)

	var storedtx Tx

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", txHash1[:])
	require.NoError(s.T(), err)

	require.Equal(s.T(), txHash1[:], storedtx.Hash)
	require.Equal(s.T(), testMerklePaths[0], storedtx.MerklePath)

	var mp store.BlockTransactionMap
	err = d.Get(&mp, "SELECT blockid, txid, pos from block_transactions_map WHERE txid=$1", storedtx.ID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), storedtx.ID, mp.TransactionID)
	require.Equal(s.T(), testBlockID, uint64(mp.BlockID))

	var storedtx2 Tx

	err = d.Get(&storedtx2, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", txHash2[:])
	require.NoError(s.T(), err)

	require.Equal(s.T(), txHash2[:], storedtx2.Hash)
	require.Equal(s.T(), testMerklePaths[1], storedtx2.MerklePath)

	var mp2 store.BlockTransactionMap
	err = d.Get(&mp2, "SELECT blockid, txid, pos from block_transactions_map WHERE txid=$1", storedtx2.ID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), storedtx2.ID, mp2.TransactionID)
	require.Equal(s.T(), testBlockID, uint64(mp2.BlockID))

}

func TestUpdateBlockTransactionsSuite(t *testing.T) {
	s := new(UpdateBlockTransactionsSuite)
	suite.Run(t, s)
}
