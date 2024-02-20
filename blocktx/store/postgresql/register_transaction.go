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

// RegisterTransaction registers a transaction in the database.
func (s *PostgreSQL) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("RegisterTransaction").AddTime(start)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:RegisterTransaction")
	defer span.Finish()

	if transaction.Hash == nil {
		return ErrRegisterTransactionMissingHash
	}

	q := `INSERT INTO transactions (hash, source) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, q, transaction.Hash[:], transaction.Source)
	if err != nil {
		return err
	}

	return nil
}

func (s *PostgreSQL) RegisterTransactions(ctx context.Context, transactions []*blocktx_api.TransactionAndSource) error {
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
	_, err := s.db.ExecContext(ctx, q, pq.Array(hashes))
	if err != nil {
		return fmt.Errorf("failed to bulk insert transactions: %v", err)
	}

	return nil
}
