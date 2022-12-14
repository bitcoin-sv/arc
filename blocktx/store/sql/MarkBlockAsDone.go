package sql

import (
	"context"
	"database/sql"
)

func (s *SQL) MarkBlockAsDone(ctx context.Context, blockId uint64) error {
	return markBlockAsDone(ctx, s.db, blockId, true)
}

func markBlockAsDone(_ctx context.Context, db *sql.DB, blockId uint64, done bool) error {
	ctx, cancel := context.WithCancel(_ctx)
	defer cancel()

	q := `
		UPDATE blocks
		SET processedyn = $1
		WHERE id = $2
	`

	if _, err := db.ExecContext(ctx, q, done, blockId); err != nil {
		return err
	}

	return nil
}
