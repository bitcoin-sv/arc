package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

// SetBlockProcessing tries to insert a record to the block processing table in order to mark a certain block as being processed by an instance. A new entry will be inserted successfully if there is no entry from any instance inserted less than `lockTime` ago and if there are less than `maxParallelProcessing` blocks currently being processed by the instance denoted by `setProcessedBy`.
func (p *PostgreSQL) SetBlockProcessing(ctx context.Context, hash *chainhash.Hash, setProcessedBy string, lockTime time.Duration, maxParallelProcessing int) (string, error) {
	// Try to set a block as being processed by this instance
	qInsert := `
		INSERT INTO blocktx.block_processing (block_hash, processed_by)
		SELECT $1, $2
		WHERE NOT EXISTS (
		  	(SELECT 1 FROM blocktx.block_processing bp WHERE bp.block_hash = $1 AND inserted_at > $3)
				UNION
			(SELECT 1 FROM blocktx.block_processing bp
			LEFT JOIN blocktx.blocks b ON b.hash = bp.block_hash
			WHERE b.processed_at IS NULL AND bp.processed_by = $2
			OFFSET $4)
		)
		RETURNING processed_by
	`

	var processedBy string
	err := p.db.QueryRowContext(ctx, qInsert, hash[:], setProcessedBy, p.now().Add(-1*lockTime), maxParallelProcessing-1).Scan(&processedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var currentlyProcessedBy string
			err = p.db.QueryRowContext(ctx, `SELECT processed_by FROM blocktx.block_processing WHERE block_hash = $1 AND inserted_at > $2 ORDER BY inserted_at DESC LIMIT 1`, hash[:], p.now().Add(-1*lockTime)).Scan(&currentlyProcessedBy)
			if err == nil {
				return currentlyProcessedBy, store.ErrBlockProcessingInProgress
			}

			return "", store.ErrBlockProcessingMaximumReached
		}

		return "", errors.Join(store.ErrFailedToSetBlockProcessing, err)
	}

	return processedBy, nil
}
