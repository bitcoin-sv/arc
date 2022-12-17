package sql

import (
	"context"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
)

// InsertTransaction registers a transaction in the database
func (s *SQL) InsertTransaction(ctx context.Context, transaction *blocktx_api.Transaction) error {
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
			ON CONFLICT DO UPDATE SET source = $2 WHERE source IS NULL
			;
		`
		args = append(args, transaction.Hash, transaction.Source)
	}

	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return err

	}

	return nil
}
