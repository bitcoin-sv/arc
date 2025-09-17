package postgresql

import (
	"database/sql"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (p *PostgreSQL) parseBlocks(rows *sql.Rows) ([]*blocktx_api.Block, error) {
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

		block.Processed = processedAt.Valid
		if processedAt.Valid {
			block.ProcessedAt = timestamppb.New(processedAt.Time)
		}

		if timestamp.Valid {
			block.Timestamp = timestamppb.New(timestamp.Time)
		}

		blocks = append(blocks, &block)
	}

	return blocks, nil
}
