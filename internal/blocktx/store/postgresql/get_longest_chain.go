package postgresql

import (
	"context"
	"database/sql"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

func (p *PostgreSQL) GetLongestChainFromHeight(ctx context.Context, height uint64) ([]*blocktx_api.Block, error) {
	q := `
		SELECT
			hash
		 ,prevhash
		 ,merkleroot
		 ,height
		 ,processed_at
		 ,orphanedyn
		 ,status
		 ,chainwork
		FROM blocktx.blocks
		WHERE height >= $1 AND status = $2
	`

	longestBlocks := make([]*blocktx_api.Block, 0)

	rows, err := p.db.QueryContext(ctx, q, height, blocktx_api.Status_LONGEST)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var block blocktx_api.Block
		var processed_at sql.NullString

		err := rows.Scan(
			&block.Hash,
			&block.PreviousHash,
			&block.MerkleRoot,
			&block.Height,
			&processed_at,
			&block.Orphaned,
			&block.Status,
			&block.Chainwork,
		)
		if err != nil {
			return nil, err
		}

		block.Processed = processed_at.Valid

		longestBlocks = append(longestBlocks, &block)
	}

	return longestBlocks, nil
}
