package sql

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/gocore"
)

var (
	ErrRegisterTransactionMissingHash = errors.New("invalid request - no hash")
)

// RegisterTransaction registers a transaction in the database.
func (s *SQL) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error {
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
