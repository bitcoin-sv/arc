package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/gocore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Todo: Revisit this test and check if it still makes sense
func TestInOut(t *testing.T) {
	ctx := context.Background()

	block := &blocktx_api.Block{
		Hash:         []byte("test block hash"),
		MerkleRoot:   []byte("test merkleroot"),
		PreviousHash: []byte("test prevhash"),
		Height:       1,
	}

	s, err := New(true, "")
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
		err = s.RegisterTransaction(ctx, txn)
		require.NoError(t, err)
	}

	err = s.RegisterTransaction(ctx, &blocktx_api.TransactionAndSource{
		Hash:   firstHash[:],
		Source: "TEST",
	})
	require.NoError(t, err)

	_, err = s.UpdateBlockTransactions(ctx, blockId, transactions, make([]string, len(transactions)))
	require.NoError(t, err)

	txns, err := getBlockTransactions(ctx, block, s.db)
	require.NoError(t, err)

	for i, txn := range txns.GetTransactions() {
		assert.Equal(t, bytes.Equal(transactions[i].GetHash(), txn.GetHash()), true)
	}

	height := uint64(1)

	err = setOrphanedForHeight(ctx, s.db, height, false)
	require.NoError(t, err)

	block2, err := getBlockForHeight(ctx, s.db, height)
	require.NoError(t, err)
	assert.Equal(t, bytes.Equal(block.GetHash(), block2.GetHash()), true)

	err = setOrphanedForHeight(ctx, s.db, height, true)
	require.NoError(t, err)

	block3, err := getBlockForHeight(ctx, s.db, height)
	require.Error(t, err)
	assert.Nil(t, block3)
}

func TestBlockNotExists(t *testing.T) {
	ctx := context.Background()

	s, err := New(true, "")
	require.NoError(t, err)

	height := uint64(1000000)

	b, err := getBlockForHeight(ctx, s.db, height)
	require.Error(t, err)
	assert.Nil(t, b)
}

func getBlockTransactions(ctx context.Context, block *blocktx_api.Block, db *sql.DB) (*blocktx_api.Transactions, error) {

	q := `
		SELECT
		 t.hash
		FROM transactions t
		INNER JOIN block_transactions_map m ON m.txid = t.id
		INNER JOIN blocks b ON m.blockid = b.id
		WHERE b.hash = $1
	`

	rows, err := db.QueryContext(ctx, q, block.GetHash())
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var hash []byte
	var transactions []*blocktx_api.Transaction

	for rows.Next() {
		err = rows.Scan(&hash)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, &blocktx_api.Transaction{
			Hash: hash,
		})
	}

	return &blocktx_api.Transactions{
		Transactions: transactions,
	}, nil
}

func setOrphanedForHeight(ctx context.Context, db *sql.DB, height uint64, orphaned bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		UPDATE blocks
		SET orphanedyn = $1
		WHERE height = $2
	`

	if _, err := db.ExecContext(ctx, q, orphaned, height); err != nil {
		return err
	}

	return nil
}

func getBlockForHeight(ctx context.Context, db *sql.DB, height uint64) (*blocktx_api.Block, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetBlockForHeight").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.prevhash
		,b.merkleroot
		,b.height
		FROM blocks b
		WHERE b.height = $1
		AND b.orphanedyn = false
	`

	var block blocktx_api.Block
	if err := db.QueryRowContext(ctx, q, height).Scan(&block.Hash, &block.PreviousHash, &block.MerkleRoot, &block.Height); err != nil {
		return nil, err
	}

	return &block, nil
}
