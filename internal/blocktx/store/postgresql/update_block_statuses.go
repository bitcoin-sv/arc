package postgresql

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
)

func (p *PostgreSQL) UpdateBlocksStatuses(ctx context.Context, blockStatusUpdates []store.BlockStatusUpdate) error {
	q := `
		UPDATE blocktx.blocks b
		SET status = updates.status, is_longest = updates.is_longest
		FROM (SELECT * FROM UNNEST($1::BYTEA[], $2::INTEGER[], $3::BOOLEAN[]) AS u(hash, status, is_longest)) AS updates
		WHERE b.hash = updates.hash
	`

	blockHashes := make([][]byte, len(blockStatusUpdates))
	statuses := make([]blocktx_api.Status, len(blockStatusUpdates))
	is_longest := make([]bool, len(blockStatusUpdates))

	for i, update := range blockStatusUpdates {
		blockHashes[i] = update.Hash
		statuses[i] = update.Status
		is_longest[i] = update.Status == blocktx_api.Status_LONGEST
	}

	_, err := p.db.ExecContext(ctx, q, pq.Array(blockHashes), pq.Array(statuses), pq.Array(is_longest))
	if err != nil {
		return errors.Join(store.ErrFailedToUpdateBlockStatuses, err)
	}

	return nil
}
