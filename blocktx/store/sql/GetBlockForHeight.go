package sql

import (
	pb "github.com/TAAL-GmbH/arc/blocktx_api"

	"context"
)

// GetBlockForHeight returns the un-orphaned block for a given height, if it exists
func (s *SQL) GetBlockForHeight(ctx context.Context, height uint64) (*pb.Block, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.header
		,b.height
		FROM blocks b
		WHERE b.height = $1
		AND b.orphanedyn = false
	`

	block := &pb.Block{}

	if err := s.db.QueryRowContext(ctx, q, height).Scan(&block.Hash, &block.Header, &block.Height); err != nil {
		return nil, err
	}

	return block, nil
}
