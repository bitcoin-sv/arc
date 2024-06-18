package postgresql

import (
	"context"
	"errors"
	"fmt"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/trace"
)

func (p *PostgreSQL) SetBlockProcessing(ctx context.Context, hash *chainhash.Hash, setProcessedBy string) (string, error) {

	// Try to set a block as being processed by this instance
	qInsert := `
		INSERT INTO block_processing (block_hash, processed_by)
		VALUES ($1 ,$2 )
		RETURNING processed_by
	`

	var processedBy string
	err := p.db.QueryRowContext(ctx, qInsert, hash[:], setProcessedBy).Scan(&processedBy)
	if err != nil {
		var pqErr *pq.Error

		if errors.As(err, &pqErr) && pqErr.Code == pq.ErrorCode("23505") {
			err = p.db.QueryRowContext(ctx, `SELECT processed_by FROM block_processing WHERE block_hash = $1`, hash[:]).Scan(&processedBy)
			if err != nil {
				return "", fmt.Errorf("failed to set block processing: %v", err)
			}

			return processedBy, store.ErrBlockProcessingDuplicateKey
		}
	}

	return processedBy, nil
}

func (p *PostgreSQL) DelBlockProcessing(ctx context.Context, hash *chainhash.Hash, processedBy string) (int64, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "InsertBlock")
		defer span.End()
	}

	q := `
		DELETE FROM block_processing WHERE block_hash = $1 AND processed_by = $2;
    `

	res, err := p.db.ExecContext(ctx, q, hash[:], processedBy)
	if err != nil {
		return 0, err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected != 1 {
		return 0, store.ErrBlockNotFound
	}

	return rowsAffected, nil
}

func (p *PostgreSQL) GetBlockHashesProcessingInProgress(ctx context.Context, processedBy string) ([]*chainhash.Hash, error) {
	// Check how many blocks this instance is currently processing
	q := `
	SELECT bp.block_hash FROM block_processing bp
	LEFT JOIN blocks b ON b.hash = bp.block_hash
	WHERE b.processed_at IS NULL AND bp.processed_by = $1;
    `

	rows, err := p.db.QueryContext(ctx, q, processedBy)
	if err != nil {
		return nil, err
	}

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
