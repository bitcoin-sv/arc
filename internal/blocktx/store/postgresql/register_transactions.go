package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) RegisterTransactions(ctx context.Context, transactions []*blocktx_api.TransactionAndSource) ([]*chainhash.Hash, error) {
	hashes := make([][]byte, len(transactions))
	for i, transaction := range transactions {
		hashes[i] = transaction.Hash
	}

	const q = `INSERT INTO transactions (hash, is_registered)
					SELECT hash, TRUE 
					FROM UNNEST ($1::BYTEA[]) as hash
				ON CONFLICT (hash) DO UPDATE 
					SET is_registered = TRUE
				RETURNING hash, inserted_at
				`

	now := p.now()
	rows, err := p.db.QueryContext(ctx, q, pq.Array(hashes))
	if err != nil {
		return nil, fmt.Errorf("failed to bulk insert transactions: %v", err)
	}

	updatedTxs := make([]*chainhash.Hash, 0)
	for rows.Next() {
		var hash []byte
		var insertedAt time.Time

		err = rows.Scan(&hash, &insertedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to get rows: %v", err)
		}

		if insertedAt.Before(now) {
			ch, _ := chainhash.NewHash(hash)
			updatedTxs = append(updatedTxs, ch)
		}
	}

	return updatedTxs, nil
}
