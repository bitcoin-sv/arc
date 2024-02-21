package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/gocore"
)

var (
	ErrRegisterTransactionMissingHash = errors.New("invalid request - no hash")
)

func (p *PostgreSQL) RegisterTransactions(ctx context.Context, transactions []*blocktx_api.TransactionAndSource) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("RegisterTransactions").AddTime(start)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:RegisterTransactions")
	defer span.Finish()

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
