package postgresql

import (
	"context"

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
		 ,status
		 ,chainwork
		FROM blocktx.blocks
		WHERE height >= $1 AND is_longest = true AND processed_at IS NOT NULL
	`

	rows, err := p.db.QueryContext(ctx, q, height)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return p.parseBlocks(rows)
}
