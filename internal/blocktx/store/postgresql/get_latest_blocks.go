package postgresql

import (
	"context"
	"database/sql"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		 ,timestamp
		FROM blocktx.blocks
		WHERE is_longest = TRUE AND processed_at IS NOT NULL ORDER BY height DESC LIMIT $1`

	rows, err := p.db.QueryContext(ctx, q, numOfBlocks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := make([]*blocktx_api.Block, 0)

	for rows.Next() {
		var block blocktx_api.Block
		var processedAt sql.NullTime
		var timestamp sql.NullTime
		err := rows.Scan(
			&block.Hash,
			&block.PreviousHash,
			&block.MerkleRoot,
			&block.Height,
			&processedAt,
			&block.Status,
			&block.Chainwork,
			&timestamp,
		)
		if err != nil {
			return nil, err
		}

		if timestamp.Valid {
			block.Timestamp = timestamppb.New(timestamp.Time.UTC())
		}

		if processedAt.Valid {
			block.ProcessedAt = timestamppb.New(processedAt.Time.UTC())
		}

		blocks = append(blocks, &block)
	}

	return blocks, nil
}
