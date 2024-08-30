package postgresql

import (
	"context"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/trace"
)

func (p *PostgreSQL) MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "MarkBlockAsDone")
		defer span.End()
	}

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
