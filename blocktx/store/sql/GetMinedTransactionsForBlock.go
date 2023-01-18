package sql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
)

// GetMinedTransactionsForBlock returns the transaction hashes for a given block hash and source
func (s *SQL) GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	qBlock := `
		SELECT
		 b.hash
		,b.height
		,b.merkleroot
		,b.prevhash
		,b.orphanedyn
		,b.processedyn
		FROM blocks b
		WHERE b.hash = $1
	`

	qTransactions := `
		SELECT
		 t.hash
		FROM transactions t
		INNER JOIN block_transactions_map m ON m.txid = t.id
		INNER JOIN blocks b ON m.blockid = b.id
		WHERE b.hash = $1
		AND t.source = $2
	`

	var block blocktx_api.Block

	if err := s.db.QueryRowContext(ctx, qBlock, blockAndSource.Hash).Scan(
		&block.Hash,
		&block.Height,
		&block.MerkleRoot,
		&block.PreviousHash,
		&block.Orphaned,
		&block.Processed,
	); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, qTransactions, blockAndSource.Hash, blockAndSource.Source)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	defer rows.Close()

	var hash []byte
	transactions := make([]*blocktx_api.Transaction, 0)

	for rows.Next() {
		err = rows.Scan(&hash)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, &blocktx_api.Transaction{
			Hash: hash,
		})
	}

	return &blocktx_api.MinedTransactions{
		Block:        &block,
		Transactions: transactions,
	}, nil
}
