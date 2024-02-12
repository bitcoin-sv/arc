package sqlite

import (
	"context"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/gocore"
)

func (s *SqLite) MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("MarkBlockAsDone").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		UPDATE blocks
		SET processed_at = CURRENT_TIMESTAMP
		,size = $1
		,tx_count = $2
		WHERE hash = $3
		`

	if _, err := s.db.ExecContext(ctx, q, size, txCount, hash[:]); err != nil {
		return err
	}

	return nil
}
