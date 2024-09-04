package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/lib/pq"
)

func (p *PostgreSQL) UpdateBlocksStatuses(ctx context.Context, hashes [][]byte, status blocktx_api.Status) error {
	q := `
		UPDATE blocktx.blocks
		SET status = $1
		WHERE hash = ANY($2)
	`

	_, err := p.db.ExecContext(ctx, q, status, pq.Array(hashes))
	if err != nil {
		return err
	}

	return nil
}
