package postgresql

import (
	"context"
	"errors"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"

	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) RegisterTransactions(ctx context.Context, txHashes [][]byte) ([]*chainhash.Hash, error) {
	const q = `
		INSERT INTO blocktx.transactions (hash, is_registered)
			SELECT hash, TRUE
			FROM UNNEST ($1::BYTEA[]) as hash
		ON CONFLICT (hash) DO UPDATE
			SET is_registered = TRUE
		RETURNING hash, inserted_at
	`

	now := p.now()
	rows, err := p.db.QueryContext(ctx, q, pq.Array(txHashes))
	if err != nil {
		return nil, errors.Join(store.ErrFailedToInsertTransactions, err)
	}
	defer rows.Close()

	updatedTxs := make([]*chainhash.Hash, 0)
	for rows.Next() {
		var hash []byte
		var insertedAt time.Time

		err = rows.Scan(&hash, &insertedAt)
		if err != nil {
			return nil, errors.Join(store.ErrFailedToGetRows, err)
		}

		if insertedAt.Before(now) {
			ch, _ := chainhash.NewHash(hash)
			updatedTxs = append(updatedTxs, ch)
		}
	}

	return updatedTxs, nil
}
