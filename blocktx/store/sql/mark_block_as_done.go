package sql

import (
	"context"
	"database/sql"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/gocore"
)

func (s *SQL) MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("MarkBlockAsDone").AddTime(start)
	}()

	return markBlockAsDone(ctx, s.db, hash, size, txCount)
}

func markBlockAsDone(_ctx context.Context, db *sql.DB, hash *chainhash.Hash, size uint64, txCount uint64) error {
	ctx, cancel := context.WithCancel(_ctx)
	defer cancel()

	q := `
		UPDATE blocks
		SET processed_at = CURRENT_TIMESTAMP
		,size = $1
		,tx_count = $2
		WHERE hash = $3
		`

	if _, err := db.ExecContext(ctx, q, size, txCount, hash[:]); err != nil {
		return err
	}

	return nil
}
