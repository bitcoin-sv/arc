package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

const BlockDistance = 2016

func (p *PostgreSQL) GetStats(ctx context.Context) (*store.Stats, error) {
	q := `
	SELECT count(*) FROM (
	SELECT unnest(ARRAY(
			SELECT a.n	
			FROM generate_series((SELECT max(height) - $1 AS block_height FROM blocktx.blocks b), (SELECT max(height) AS block_height FROM blocktx.blocks b)) AS a(n)
		)) AS block_heights) AS bl
	LEFT JOIN blocktx.blocks blks ON blks.height = bl.block_heights
	WHERE blks.height IS NULL;
	`

	stats := &store.Stats{}

	err := p.db.QueryRowContext(ctx, q, BlockDistance).Scan(
		&stats.CurrentNumOfBlockGaps,
	)
	if err != nil {
		return nil, err
	}

	return stats, nil
}
