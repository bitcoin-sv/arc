package postgresql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

func (p *PostgreSQL) GetBlockByHeight(ctx context.Context, height uint64, status blocktx_api.Status) (*blocktx_api.Block, error) {
	q := `
		SELECT
			b.hash,
			b.prevhash,
			b.merkleroot,
			b.height,
			b.processed_at,
			b.orphanedyn,
			b.status,
			b.chainwork
		FROM blocktx.blocks b
		WHERE b.height = $1 AND b.status = $2
	`

	var block blocktx_api.Block

	var processed_at sql.NullString

	if err := p.db.QueryRowContext(ctx, q, height, status).Scan(
		&block.Hash,
		&block.PreviousHash,
		&block.MerkleRoot,
		&block.Height,
		&processed_at,
		&block.Orphaned,
		&block.Status,
		&block.Chainwork,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrBlockNotFound
		}
		return nil, err
	}

	block.Processed = processed_at.Valid

	return &block, nil
}
