package postgresql

import (
	"context"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
	ctx, span := p.startTracing(ctx, "MarkBlockAsDone")
	defer p.endTracing(span)

	q := `
		UPDATE blocktx.blocks
		SET processed_at = $4,
				size = $1,
				tx_count = $2
		WHERE hash = $3
	`

	if _, err := p.db.ExecContext(ctx, q, size, txCount, hash[:], p.now()); err != nil {
		return err
	}

	return nil
}
