package sql

import (
	"context"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/ordishs/gocore"
)

// InsertTransaction registers a transaction in the database
func (s *SQL) InsertTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("InsertTransaction").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var q string
	var args []interface{}

	if transaction.Source == "" {
		q = `
			INSERT INTO transactions (hash) VALUES ($1)
			ON CONFLICT DO NOTHING
			;
		`
		args = append(args, transaction.Hash)

	} else {
		q = `
			INSERT INTO transactions (hash, source) VALUES ($1, $2)
			ON CONFLICT(hash) DO UPDATE SET source = $2
			;
		`
		args = append(args, transaction.Hash, transaction.Source)
	}

	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return err

	}

	return nil
}
