package sql

import (
	"context"

	pb "github.com/TAAL-GmbH/arc/blocktx_api"
)

// InsertTransaction registers a transaction in the database
func (s *SQL) InsertTransaction(ctx context.Context, transaction *pb.Transaction) error {
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
			INSERT INTO transactions (hash) VALUES ($1)
			ON CONFLICT DO UPDATE SET source = $2 WHERE source IS NULL -- TODO discuss this
			;
		`
		args = append(args, transaction.Hash, transaction.Source)
	}

	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return err

	}

	return nil
}
