package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

func (p *PostgreSQL) LatestBlocks(ctx context.Context, numOfBlocks uint64) ([]*blocktx_api.Block, error) {
	q := `
		SELECT
			hash
		 ,prevhash
		 ,merkleroot
		 ,height
		 ,processed_at
		 ,status
		 ,chainwork
		FROM blocktx.blocks
		WHERE is_longest = true AND processed_at IS NOT NULL order by height desc LIMIT $1`

	rows, err := p.db.QueryContext(ctx, q, numOfBlocks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := make([]*blocktx_api.Block, 0)

	for rows.Next() {
		var block blocktx_api.Block
		err := rows.Scan(
			&block.Hash,
			&block.PreviousHash,
			&block.MerkleRoot,
			&block.Height,
			&block.ProcessedAt,
			&block.Status,
			&block.Chainwork,
		)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, &block)
	}

	return blocks, nil
}
