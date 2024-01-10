package sql

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/go-testfixtures/testfixtures/v3"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	. "github.com/bitcoin-sv/arc/database_testing"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GetTransactionBlocksSuite struct {
	BlockTXDBTestSuite
}

func (s *GetTransactionBlocksSuite) Test() {

	db, err := sql.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	st := &SQL{db: db, engine: postgresEngine}

	fixtures, err := testfixtures.New(
		testfixtures.Database(db),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures/get_transaction_blocks"), // The directory containing the YAML files
	)
	require.NoError(s.T(), err)

	err = fixtures.Load()
	require.NoError(s.T(), err)
	ctx := context.Background()

	txHash1, err := chainhash.NewHashFromStr("b0926372c449731ffb84b7d2f808087d4b5e6e26aafee872232bbcd5a5854e16")
	require.NoError(s.T(), err)

	blockHash, err := chainhash.NewHashFromStr("000000000000000001d29fef90f0f86eb7a99f043b994c7e363e0aa72db05862")
	require.NoError(s.T(), err)

	b, err := st.GetTransactionBlocks(ctx, &blocktx_api.Transactions{
		Transactions: []*blocktx_api.Transaction{
			{
				Hash: txHash1[:],
			},
		},
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), blockHash[:], b.TransactionBlocks[0].BlockHash)
	require.Equal(s.T(), uint64(826481), b.TransactionBlocks[0].BlockHeight)
	require.Equal(s.T(), txHash1[:], b.TransactionBlocks[0].TransactionHash)

	txHash2, err := chainhash.NewHashFromStr("3b8c1676470f44043069f66fdc0d5df9bdad1865a8f03b8da1268359802b7376")
	require.NoError(s.T(), err)
	b, err = st.GetTransactionBlocks(ctx, &blocktx_api.Transactions{
		Transactions: []*blocktx_api.Transaction{
			{
				Hash: txHash2[:],
			},
		},
	})
	require.NoError(s.T(), err)
	require.Nil(s.T(), b.TransactionBlocks[0].BlockHash)
	require.Equal(s.T(), uint64(0), b.TransactionBlocks[0].BlockHeight)
	require.Equal(s.T(), txHash2[:], b.TransactionBlocks[0].TransactionHash)
}

func TestGetTransactionBlocksIntegration(t *testing.T) {
	s := new(GetTransactionBlocksSuite)
	suite.Run(t, s)
}

func TestGetTransactionBlocks(t *testing.T) {
	txHash1, err := chainhash.NewHashFromStr("181fcd0be5a1742aabd594a5bfd5a1e7863a4583290da72fb2a896dfa824645c")
	require.NoError(t, err)
	txHash2, err := chainhash.NewHashFromStr("2e5c318f7f2e2e80e484ca1f00f1b7bee95a33a848de572a304b973ff2b0b35b")
	require.NoError(t, err)
	txHash3, err := chainhash.NewHashFromStr("82b0a66c5dcbd0f6f6f99e2bf766e1d40b04c175c01ee87f1abc36136e511a7e")
	require.NoError(t, err)

	blockHash1, err := chainhash.NewHashFromStr("0000000000000000079eeb9eded0cdbcb7241d116a5f6baff48e5c237ab360bf")
	require.NoError(t, err)

	tt := []struct {
		name                string
		sqlmockExpectations func(mock sqlmock.Sqlmock)

		expectedErrorStr string
	}{
		{
			name: "success",
			sqlmockExpectations: func(mock sqlmock.Sqlmock) {
				query := `
				SELECT
				b.hash, b.height, t.hash
				FROM blocks b
				INNER JOIN block_transactions_map m ON m.blockid = b.id
				INNER JOIN transactions t ON m.txid = t.id
				WHERE t.hash in ('\x5c6424a8df96a8b22fa70d2983453a86e7a1d5bfa594d5ab2a74a1e50bcd1f18','\x5bb3b0f23f974b302a57de48a8335ae9beb7f1001fca84e4802e2e7f8f315c2e','\x7e1a516e1336bc1a7fe81ec075c1040bd4e166f72b9ef9f6f6d0cb5d6ca6b082')
				AND b.orphanedyn = FALSE`

				mock.ExpectQuery(query).WillReturnRows(
					sqlmock.NewRows([]string{"hash", "height", "hash"}).AddRow(
						blockHash1.CloneBytes(),
						1,
						txHash1.CloneBytes(),
					),
				)
			},
		},
		{
			name: "query fails",
			sqlmockExpectations: func(mock sqlmock.Sqlmock) {
				query := `
				SELECT
				b.hash, b.height, t.hash
				FROM blocks b
				INNER JOIN block_transactions_map m ON m.blockid = b.id
				INNER JOIN transactions t ON m.txid = t.id
				WHERE t.hash = ANY($1)
				AND b.orphanedyn = FALSE`

				mock.ExpectQuery(query).WillReturnError(errors.New("db connection error"))
			},
			expectedErrorStr: "db connection error",
		},
		{
			name: "scanning fails",
			sqlmockExpectations: func(mock sqlmock.Sqlmock) {
				query := `
				SELECT
				b.hash, b.height, t.hash
				FROM blocks b
				INNER JOIN block_transactions_map m ON m.blockid = b.id
				INNER JOIN transactions t ON m.txid = t.id
				WHERE t.hash = ANY($1)
				AND b.orphanedyn = FALSE`

				mock.ExpectQuery(query).WillReturnRows(
					sqlmock.NewRows([]string{"hash", "height", "hash"}).AddRow(
						blockHash1.CloneBytes(),
						"not a number",
						txHash1.CloneBytes(),
					),
				)
			},
			expectedErrorStr: "Scan error on column index 1",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)

			defer db.Close()

			dbEngine := SQL{
				db:     db,
				engine: "postgres",
			}

			tc.sqlmockExpectations(mock)

			transactionBlocks, err := dbEngine.GetTransactionBlocks(context.Background(), &blocktx_api.Transactions{
				Transactions: []*blocktx_api.Transaction{
					{
						Hash: txHash1.CloneBytes(),
					},
					{
						Hash: txHash2.CloneBytes(),
					},
					{
						Hash: txHash3.CloneBytes(),
					},
				},
			})
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			err = mock.ExpectationsWereMet()
			require.NoError(t, err)

			require.Equal(t, blockHash1.CloneBytes(), transactionBlocks.GetTransactionBlocks()[0].GetBlockHash())
			require.Equal(t, uint64(1), transactionBlocks.GetTransactionBlocks()[0].GetBlockHeight())
			require.Equal(t, txHash1.CloneBytes(), transactionBlocks.GetTransactionBlocks()[0].GetTransactionHash())
		})
	}
}
