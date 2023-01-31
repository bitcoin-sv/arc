package sql

import (
	"context"
	"database/sql"

	"github.com/ordishs/gocore"
)

func (s *SQL) MarkBlockAsDone(ctx context.Context, hash []byte, size uint64, txCount uint64) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("MarkBlockAsDone").AddTime(start)
	}()

	return markBlockAsDone(ctx, s.db, hash, size, txCount, true)
}

func markBlockAsDone(_ctx context.Context, db *sql.DB, hash []byte, size uint64, txCount uint64, done bool) error {
	ctx, cancel := context.WithCancel(_ctx)
	defer cancel()

	var q string

	if done {
		q = `
			UPDATE blocks
			SET processed_at = CURRENT_TIMESTAMP
			,size = $1
			,tx_count = $2
			WHERE hash = $3
		`

		if _, err := db.ExecContext(ctx, q, size, txCount, hash); err != nil {
			return err
		}
	} else {
		q = `
			UPDATE blocks
			SET processed_at = NULL
			,size = NULL
			,tx_count = NULL
			WHERE hash = $1
		`
		if _, err := db.ExecContext(ctx, q, hash); err != nil {
			return err
		}
	}

	return nil
}
