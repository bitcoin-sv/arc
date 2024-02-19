package sql

import (
	"bytes"
	"context"
	"encoding/hex"
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
		db:                        db,
		engine:                    postgresEngine,
		maxPostgresBulkInsertRows: 5,
	}

	fixtures, err := testfixtures.New(
		testfixtures.Database(db.DB),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures/update_block_transactions"),
	)
	require.NoError(s.T(), err)

	err = fixtures.Load()
	require.NoError(s.T(), err)
	testMerklePaths := []string{"test1", "test2", "test3"}
	testBlockID := uint64(9736)

	txHash1 := createTxHash(s, "76732b80598326a18d3bf0a86518adbdf95d0ddc6ff6693004440f4776168c3b")
	txHash2 := createTxHash(s, "164e85a5d5bc2b2372e8feaa266e5e4b7d0808f8d2b784fb1f7349c4726392b0")

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

	updateResult, err := st.UpdateBlockTransactions(context.Background(), testBlockID, []*blocktx_api.TransactionAndSource{
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

	require.True(s.T(), bytes.Equal(txHash1[:], updateResult[0].TxHash))
	require.Equal(s.T(), testMerklePaths[0], updateResult[0].MerklePath)

	require.True(s.T(), bytes.Equal(txHash2[:], updateResult[1].TxHash))
	require.Equal(s.T(), testMerklePaths[1], updateResult[1].MerklePath)

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

	// update exceeds max batch size

	txHash3 := createTxHash(s, "b4201cc6fc5768abff14adf75042ace6061da9176ee5bb943291b9ba7d7f5743")
	txHash4 := createTxHash(s, "37bd6c87927e75faeb3b3c939f64721cda48e1bb98742676eebe83aceee1a669")
	txHash5 := createTxHash(s, "952f80e20a0330f3b9c2dfd1586960064e797218b5c5df665cada221452c17eb")
	txHash6 := createTxHash(s, "861a281b27de016e50887288de87eab5ca56a1bb172cdff6dba965474ce0f608")
	txHash7 := createTxHash(s, "9421cc760c5405af950a76dc3e4345eaefd4e7322f172a3aee5e0ddc7b4f8313")
	txHash8 := createTxHash(s, "8b7d038db4518ac4c665abfc5aeaacbd2124ad8ca70daa8465ed2c4427c41b9b")

	_, err = st.UpdateBlockTransactions(context.Background(), testBlockID, []*blocktx_api.TransactionAndSource{
		{
			Hash: txHash3[:],
		},
		{
			Hash: txHash4[:],
		},
		{
			Hash: txHash5[:],
		},
		{
			Hash: txHash6[:],
		},
		{
			Hash: txHash7[:],
		},
		{
			Hash: txHash8[:],
		},
	}, []string{"test1", "test2", "test3", "test4", "test5", "test6"})
	require.NoError(s.T(), err)
}

func createTxHash(s *UpdateBlockTransactionsSuite, hashString string) *chainhash.Hash {
	hash, err := hex.DecodeString(hashString)
	require.NoError(s.T(), err)
	txHash, err := chainhash.NewHash(hash)
	require.NoError(s.T(), err)

	return txHash
}

func TestUpdateBlockTransactionsSuite(t *testing.T) {
	s := new(UpdateBlockTransactionsSuite)
	suite.Run(t, s)
}
