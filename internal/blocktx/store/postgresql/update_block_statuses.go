package postgresql

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
)

func (p *PostgreSQL) UpdateBlocksStatuses(ctx context.Context, blockStatusUpdates []store.BlockStatusUpdate) error {
	q := `
		UPDATE blocktx.blocks b
		SET status = updates.status, is_longest = updates.is_longest
		FROM (
			SELECT * FROM UNNEST($1::BYTEA[], $2::INTEGER[], $3::BOOLEAN[]) AS u(hash, status, is_longest)
			WHERE is_longest = $4
		) AS updates
		WHERE b.hash = updates.hash
	`

	blockHashes := make([][]byte, len(blockStatusUpdates))
	statuses := make([]blocktx_api.Status, len(blockStatusUpdates))
	isLongest := make([]bool, len(blockStatusUpdates))

	for i, update := range blockStatusUpdates {
		blockHashes[i] = update.Hash
		statuses[i] = update.Status
		isLongest[i] = update.Status == blocktx_api.Status_LONGEST
		fmt.Println("shota block status update", hex.EncodeToString(update.Hash), update.Status, update.Status == blocktx_api.Status_LONGEST)
	}

	tx, err := p.db.Begin()
	if err != nil {
		return errors.Join(store.ErrFailedToUpdateBlockStatuses, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// first update blocks that are changing statuses to non-LONGEST
	_, err = tx.ExecContext(ctx, q, pq.Array(blockHashes), pq.Array(statuses), pq.Array(isLongest), false)
	if err != nil {
		return errors.Join(store.ErrFailedToUpdateBlockStatuses, err)
	}

	// then update blocks that are changing statuses to LONGEST
	_, err = tx.ExecContext(ctx, q, pq.Array(blockHashes), pq.Array(statuses), pq.Array(isLongest), true)
	if err != nil {
		return errors.Join(store.ErrFailedToUpdateBlockStatuses, err)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Join(store.ErrFailedToUpdateBlockStatuses, err)
	}

	return nil
}
