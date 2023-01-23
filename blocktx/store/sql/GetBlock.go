package sql

import (
	"database/sql"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"

	"context"
)

func (s *SQL) GetBlock(ctx context.Context, hash []byte) (*blocktx_api.Block, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.prevhash
		,b.merkleroot
		,b.height
		,b.processed_at
		,b.orphanedyn
		FROM blocks b
		WHERE b.hash = $1
	`

	var block blocktx_api.Block

	var processed_at sql.NullString

	if err := s.db.QueryRowContext(ctx, q, hash).Scan(
		&block.Hash,
		&block.PreviousHash,
		&block.MerkleRoot,
		&block.Height,
		&processed_at,
		&block.Orphaned,
	); err != nil {
		return nil, err
	}

	block.Processed = processed_at.Valid

	return &block, nil
}
