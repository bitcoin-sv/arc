package postgresql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

func (p *PostgreSQL) GetMinedTransaction(ctx context.Context, hash []byte) ([]byte, uint64, string, error) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.height
		,t.merkle_path
		FROM transactions as t 
		JOIN block_transactions_map ON t.id = block_transactions_map.txid
		JOIN blocks as b ON block_transactions_map.blockid = b.id
		WHERE t.hash = $1
	`
	var blockHash []byte
	var blockHeight uint64
	var merklePath string

	if err := p.db.QueryRowContext(ctx, q, hash).Scan(
		&blockHash,
		&blockHeight,
		&merklePath,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) || merklePath == "" || blockHeight == 0 {
			return nil, 0, "", store.ErrBlockNotFound
		}
		return nil, 0, "", err
	}

	return blockHash, blockHeight, merklePath, nil
}
