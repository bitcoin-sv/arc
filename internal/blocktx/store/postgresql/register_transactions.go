package postgresql

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/lib/pq"
)

func (p *PostgreSQL) RegisterTransactions(ctx context.Context, transactions []*blocktx_api.TransactionAndSource) error {

	hashes := make([][]byte, len(transactions))
	booleans := make([]bool, len(transactions))

	for i, transaction := range transactions {
		hashes[i] = transaction.Hash
		booleans[i] = true
	}

	q := `
			INSERT INTO transactions (hash, is_registered)
			SELECT * FROM UNNEST ($1::BYTEA[], $2::BOOLEAN[])
			ON CONFLICT DO NOTHING
			`
	_, err := p.db.ExecContext(ctx, q, pq.Array(hashes), pq.Array(booleans))
	if err != nil {
		return fmt.Errorf("failed to bulk insert transactions: %v", err)
	}

	return nil
}
