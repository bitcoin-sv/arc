package sql

import (
	"context"
	"database/sql"
)

func (s *SQL) MarkBlockAsDone(ctx context.Context, hash []byte) error {
	return markBlockAsDone(ctx, s.db, hash, true)
}

func markBlockAsDone(_ctx context.Context, db *sql.DB, hash []byte, done bool) error {
	ctx, cancel := context.WithCancel(_ctx)
	defer cancel()

	q := `
		UPDATE blocks
		SET processedyn = $1
		WHERE hash = $2
	`

	if _, err := db.ExecContext(ctx, q, done, hash); err != nil {
		return err
	}

	return nil
}
