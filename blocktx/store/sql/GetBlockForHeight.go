package sql

import (
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/ordishs/gocore"

	"context"
)

// GetBlockForHeight returns the un-orphaned block for a given height, if it exists
func (s *SQL) GetBlockForHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetBlockForHeight").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.prevhash
		,b.merkleroot
		,b.height
		FROM blocks b
		WHERE b.height = $1
		AND b.orphanedyn = false
	`

	block := &blocktx_api.Block{}

	if err := s.db.QueryRowContext(ctx, q, height).Scan(&block.Hash, &block.PreviousHash, &block.MerkleRoot, &block.Height); err != nil {
		return nil, err
	}

	return block, nil
}
