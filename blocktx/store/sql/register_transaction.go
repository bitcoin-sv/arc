package sql

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/lib/pq"
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

func (s *SQL) RegisterTransactions(ctx context.Context, transactions []*blocktx_api.TransactionAndSource) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("RegisterTransactions").AddTime(start)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:RegisterTransactions")
	defer span.Finish()

	txn, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		_ = txn.Rollback()
	}()

	stmt, err := txn.Prepare(
		pq.CopyIn(
			"transactions", // table name
			"hash",
		),
	)
	if err != nil {
		return err
	}

	for _, transaction := range transactions {
		if _, err := stmt.Exec(
			transaction.Hash,
		); err != nil {
			stmt.Close()
			return err
		}
	}

	if err = stmt.Close(); err != nil {
		return err
	}

	if err := txn.Commit(); err != nil {
		return err
	}

	return nil
}
