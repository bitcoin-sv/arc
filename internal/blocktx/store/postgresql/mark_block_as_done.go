package postgresql

import (
	"context"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/tracing"
)

func (p *PostgreSQL) MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
	ctx, span := tracing.StartTracing(ctx, "MarkBlockAsDone", p.tracingEnabled, p.attributes...)
	defer tracing.EndTracing(span)

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
