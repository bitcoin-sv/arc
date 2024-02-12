package postgresql

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/database_testing"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RegisterTransactionSuite struct {
	database_testing.BlockTXDBTestSuite
}

func (s *RegisterTransactionSuite) Test() {
	pstore, err := NewPostgresStore(database_testing.DefaultParams)
	require.NoError(s.T(), err)

	err = pstore.RegisterTransaction(context.Background(), &blocktx_api.TransactionAndSource{})
	require.ErrorIs(s.T(), err, ErrRegisterTransactionMissingHash)

	tx := &blocktx_api.TransactionAndSource{
		Hash: []byte(database_testing.GetRandomBytes()),
	}
	err = pstore.RegisterTransaction(context.Background(), tx)
	require.NoError(s.T(), err)

	d, err := sqlx.Open("postgres", database_testing.DefaultParams.String())
	require.NoError(s.T(), err)

	var storedtx Tx

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(tx.GetHash()))
	require.NoError(s.T(), err)

	require.Equal(s.T(), tx.GetHash(), storedtx.Hash)

}

func TestRegisterTransactionSuite(t *testing.T) {
	s := new(RegisterTransactionSuite)
	suite.Run(t, s)
}

type RegisterTransactionsSuite struct {
	database_testing.BlockTXDBTestSuite
}

func (s *RegisterTransactionsSuite) Test() {
	pstore, err := NewPostgresStore(database_testing.DefaultParams)
	require.NoError(s.T(), err)

	err = pstore.RegisterTransaction(context.Background(), &blocktx_api.TransactionAndSource{})
	require.ErrorIs(s.T(), err, ErrRegisterTransactionMissingHash)

	// Register new transactions
	txs := []*blocktx_api.TransactionAndSource{
		{
			Hash: []byte(database_testing.GetRandomBytes()),
		},
		{
			Hash: []byte(database_testing.GetRandomBytes()),
		},
		{
			Hash: []byte(database_testing.GetRandomBytes()),
		},
		{
			Hash: []byte(database_testing.GetRandomBytes()),
		},
	}
	err = pstore.RegisterTransactions(context.Background(), txs)
	require.NoError(s.T(), err)

	d, err := sqlx.Open("postgres", database_testing.DefaultParams.String())
	require.NoError(s.T(), err)

	var storedtx Tx
	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs[0].GetHash()))
	require.NoError(s.T(), err)

	require.Equal(s.T(), txs[0].GetHash(), storedtx.Hash)

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs[1].GetHash()))
	require.NoError(s.T(), err)

	require.Equal(s.T(), txs[1].GetHash(), storedtx.Hash)

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs[2].GetHash()))
	require.NoError(s.T(), err)

	require.Equal(s.T(), txs[2].GetHash(), storedtx.Hash)

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs[3].GetHash()))
	require.NoError(s.T(), err)

	require.Equal(s.T(), txs[3].GetHash(), storedtx.Hash)

	// Register transactions which are partly already registered
	txs2 := []*blocktx_api.TransactionAndSource{
		txs[0],
		txs[2],
		{
			Hash: []byte(database_testing.GetRandomBytes()),
		},
		{
			Hash: []byte(database_testing.GetRandomBytes()),
		},
	}
	err = pstore.RegisterTransactions(context.Background(), txs2)
	require.NoError(s.T(), err)

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs2[2].GetHash()))
	require.NoError(s.T(), err)

	require.Equal(s.T(), txs2[2].GetHash(), storedtx.Hash)

	err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs2[3].GetHash()))
	require.NoError(s.T(), err)

	require.Equal(s.T(), txs2[3].GetHash(), storedtx.Hash)

}

func TestRegisterTransactionsSuite(t *testing.T) {
	s := new(RegisterTransactionsSuite)
	suite.Run(t, s)
}
