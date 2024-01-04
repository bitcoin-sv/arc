package sql

import (
	"context"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/ordishs/gocore"
)

func (s *SQL) GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetLastProcessedBlock").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.prevhash
		,b.merkleroot
		,b.height
		,b.orphanedyn
		FROM blocks b
		WHERE b.processed_at IS NOT NULL
		ORDER BY b.height DESC
		LIMIT 1
	`

	block := &blocktx_api.Block{}

	if err := s.db.QueryRowContext(ctx, q).Scan(&block.Hash, &block.PreviousHash, &block.MerkleRoot, &block.Height, &block.Orphaned); err != nil {
		return nil, err
	}

	return block, nil
}
