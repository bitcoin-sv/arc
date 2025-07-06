package postgresql

import (
	"context"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

func (p *PostgreSQL) BlocksSince(ctx context.Context, since time.Time) ([]*blocktx_api.Block, error) {
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
		WHERE processed_at > $1 AND is_longest = true AND processed_at IS NOT NULL`

	rows, err := p.db.QueryContext(ctx, q, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return p.parseBlocks(rows)
}
