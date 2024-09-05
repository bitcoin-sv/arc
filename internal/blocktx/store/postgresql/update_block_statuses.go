package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
)

func (p *PostgreSQL) UpdateBlocksStatuses(ctx context.Context, blockStatusUpdates []store.BlockStatusUpdate) error {
	q := `
		UPDATE blocktx.blocks b
		SET status = updates.status
		FROM (SELECT * FROM UNNEST($1::BYTEA[], $2::INTEGER[]) AS u(hash, status)) AS updates
		WHERE b.hash = updates.hash
	`

	blockHashes := make([][]byte, len(blockStatusUpdates))
	statuses := make([]blocktx_api.Status, len(blockStatusUpdates))

	for i, update := range blockStatusUpdates {
		blockHashes[i] = update.Hash
		statuses[i] = update.Status
	}

	_, err := p.db.ExecContext(ctx, q, pq.Array(blockHashes), pq.Array(statuses))
	if err != nil {
		return err
	}

	return nil
}
