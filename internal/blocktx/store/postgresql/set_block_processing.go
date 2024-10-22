package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/trace"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

func (p *PostgreSQL) SetBlockProcessing(ctx context.Context, hash *chainhash.Hash, setProcessedBy string) (string, error) {
	// Try to set a block as being processed by this instance
	qInsert := `
		INSERT INTO blocktx.block_processing (block_hash, processed_by)
		VALUES ($1, $2)
		RETURNING processed_by
	`

	var processedBy string
	err := p.db.QueryRowContext(ctx, qInsert, hash[:], setProcessedBy).Scan(&processedBy)
	if err != nil {
		var pqErr *pq.Error

		// Error 23505 is: "duplicate key violates unique constraint"
		if errors.As(err, &pqErr) && pqErr.Code == pq.ErrorCode("23505") {
			err = p.db.QueryRowContext(ctx, `SELECT processed_by FROM blocktx.block_processing WHERE block_hash = $1`, hash[:]).Scan(&processedBy)
			if err != nil {
				return "", errors.Join(store.ErrFailedToSetBlockProcessing, err)
			}

			return processedBy, store.ErrBlockProcessingDuplicateKey
		}
	}

	return processedBy, nil
}

func (p *PostgreSQL) SetBlockProcessingNew(ctx context.Context, hash *chainhash.Hash, setProcessedBy string) (string, error) {
	// Try to set a block as being processed by this instance
	qInsert := `
		INSERT INTO blocktx.block_processing (block_hash, processed_by)
			SELECT $1, $2
			WHERE NOT EXISTS (
				SELECT * FROM blocktx.block_processing bp
				WHERE bp.block_hash = $1 AND bp.inserted_at > $3
			)
		RETURNING processed_by;
	`

	tenMinAgo := p.now().Add(-10 * time.Minute) // Todo: pass as argument in function

	var processedBy string
	err := p.db.QueryRowContext(ctx, qInsert, hash[:], setProcessedBy, tenMinAgo).Scan(&processedBy)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", errors.Join(store.ErrFailedToSetBlockProcessing, err)
	}

	if processedBy == "" {
		// no result means another blocktx is currently processing the block => get the name of the blocktx instance which is currently processing the block
		err = p.db.QueryRowContext(ctx, `SELECT processed_by FROM blocktx.block_processing WHERE block_hash = $1 ORDER BY inserted_at DESC LIMIT 1`, hash[:]).Scan(&processedBy) // ORDER BY my_id DESC LIMIT 1
		if err != nil {
			return "", errors.Join(store.ErrFailedToSetBlockProcessing, err)
		}

		return processedBy, store.ErrBlockProcessingDuplicateKey
	}

	return processedBy, nil
}

func (p *PostgreSQL) DelBlockProcessing(ctx context.Context, hash *chainhash.Hash, processedBy string) (int64, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "DelBlockProcessing")
		defer span.End()
	}

	q := `
		DELETE FROM blocktx.block_processing WHERE block_hash = $1 AND processed_by = $2;
  `

	res, err := p.db.ExecContext(ctx, q, hash[:], processedBy)
	if err != nil {
		return 0, err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return 0, store.ErrBlockNotFound
	}

	return rowsAffected, nil
}

func (p *PostgreSQL) GetBlockHashesProcessingInProgress(ctx context.Context, processedBy string) ([]*chainhash.Hash, error) {
	// Check how many blocks this instance is currently processing
	q := `
		SELECT bp.block_hash FROM blocktx.block_processing bp
		LEFT JOIN blocktx.blocks b ON b.hash = bp.block_hash
		WHERE b.processed_at IS NULL AND bp.processed_by = $1;
	`

	rows, err := p.db.QueryContext(ctx, q, processedBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := make([]*chainhash.Hash, 0)

	for rows.Next() {
		var hash []byte

		err = rows.Scan(&hash)
		if err != nil {
			return nil, err
		}

		txHash, err := chainhash.NewHash(hash)
		if err != nil {
			return nil, err
		}

		hashes = append(hashes, txHash)
	}

	return hashes, nil
}
