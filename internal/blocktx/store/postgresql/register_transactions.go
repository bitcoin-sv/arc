package postgresql

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/lib/pq"
)

func (p *PostgreSQL) RegisterTransactions(ctx context.Context, transactions []*blocktx_api.TransactionAndSource) error {

	hashes := make([][]byte, len(transactions))

	for i, transaction := range transactions {
		hashes[i] = transaction.Hash
	}

	q := `
			INSERT INTO transactions (hash)
			SELECT * FROM UNNEST ($1::BYTEA[])
			ON CONFLICT DO NOTHING
			`
	_, err := p.db.ExecContext(ctx, q, pq.Array(hashes))
	if err != nil {
		return fmt.Errorf("failed to bulk insert transactions: %v", err)
	}

	return nil
}
