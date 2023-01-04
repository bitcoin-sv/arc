package sql

import (
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"

	"context"
)

func (s *SQL) GetBlock(ctx context.Context, hash []byte) (*blocktx_api.Block, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.header
		,b.height
		,b.processedyn
		,b.orphanedyn
		FROM blocks b
		WHERE b.hash = $1
	`

	var block blocktx_api.Block

	if err := s.db.QueryRowContext(ctx, q, hash).Scan(
		&block.Hash,
		&block.Header,
		&block.Height,
		&block.Processed,
		&block.Orphaned,
	); err != nil {
		return nil, err
	}

	return &block, nil
}
