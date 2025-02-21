package postgresql

import (
	"context"
	"errors"

	"github.com/lib/pq"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

func (p *PostgreSQL) RegisterTransactions(ctx context.Context, txHashes [][]byte) (int64, error) {
	const q = `
		INSERT INTO blocktx.registered_transactions (hash)
			SELECT hash
			FROM UNNEST ($1::BYTEA[]) as hash
		ON CONFLICT (hash) DO NOTHING
	`

	res, err := p.db.ExecContext(ctx, q, pq.Array(txHashes))
	if err != nil {
		return 0, errors.Join(store.ErrFailedToInsertTransactions, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}

	return rowsAffected, nil
}
