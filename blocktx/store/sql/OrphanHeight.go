package sql

import (
	"context"
	"database/sql"
)

func (s *SQL) OrphanHeight(ctx context.Context, height uint64) error {
	return setOrphanedForHeight(ctx, s.db, height, true)
}

func (s *SQL) SetOrphanHeight(ctx context.Context, height uint64, orphaned bool) error {
	return setOrphanedForHeight(ctx, s.db, height, orphaned)
}

func setOrphanedForHeight(ctx context.Context, db *sql.DB, height uint64, orphaned bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		UPDATE blocks
		SET orphanedyn = $1
		WHERE height = $2
	`

	if _, err := db.ExecContext(ctx, q, orphaned, height); err != nil {
		return err
	}

	return nil
}
