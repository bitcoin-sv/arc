package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

func (p *PostgreSQL) GetStats(ctx context.Context, retentionDays int) (*store.Stats, error) {
	const (
		hoursPerDay   = 24
		blocksPerHour = 6
	)
	heightRange := retentionDays * hoursPerDay * blocksPerHour

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

	err := p.db.QueryRowContext(ctx, q, heightRange).Scan(
		&stats.CurrentNumOfBlockGaps,
	)
	if err != nil {
		return nil, err
	}

	return stats, nil
}
