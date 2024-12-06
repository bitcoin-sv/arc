package postgresql

import (
	"database/sql"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

func (p *PostgreSQL) parseBlocks(rows *sql.Rows) ([]*blocktx_api.Block, error) {
	blocks := make([]*blocktx_api.Block, 0)

	for rows.Next() {
		var block blocktx_api.Block
		var processedAt sql.NullString

		err := rows.Scan(
			&block.Hash,
			&block.PreviousHash,
			&block.MerkleRoot,
			&block.Height,
			&processedAt,
			&block.Status,
			&block.Chainwork,
		)
		if err != nil {
			return nil, err
		}

		block.Processed = processedAt.Valid

		blocks = append(blocks, &block)
	}

	return blocks, nil
}
