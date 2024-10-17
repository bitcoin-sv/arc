package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

func (p *PostgreSQL) GetOrphanedChainUpFromHash(ctx context.Context, hash []byte) ([]*blocktx_api.Block, error) {
	q := `
		WITH RECURSIVE orphans AS (
			SELECT
				hash
				,prevhash
				,merkleroot
				,height
				,processed_at
				,orphanedyn
				,status
				,chainwork
			FROM blocktx.blocks WHERE prevhash = $1 AND status = $2
			UNION ALL
			SELECT
				b.hash
				,b.prevhash
				,b.merkleroot
				,b.height
				,b.processed_at
				,b.orphanedyn
				,b.status
				,b.chainwork
			FROM blocktx.blocks b JOIN orphans o ON b.prevhash = o.hash AND b.status = $2
		)
		SELECT
			hash
		 ,prevhash
		 ,merkleroot
		 ,height
		 ,processed_at
		 ,orphanedyn
		 ,status
		 ,chainwork
		FROM orphans
	`

	rows, err := p.db.QueryContext(ctx, q, hash, blocktx_api.Status_ORPHANED)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return p.parseBlocks(rows)
}
