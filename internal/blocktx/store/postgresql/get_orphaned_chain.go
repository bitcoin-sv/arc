package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

// GetOrphanedChainUpFromHash is a function that recursively searches for blocks marked
// as ORPHANED from the given hash - up to the tip of orphaned chain of blocks.
//
// It searches for the block whose prevhash matches the hash of the given block,
// and then repeats that recursively for each newly found orphaned block until
// it has the entire orphaned chain.
func (p *PostgreSQL) GetOrphanedChainUpFromHash(ctx context.Context, hash []byte) ([]*blocktx_api.Block, error) {
	// The way this query works, is that the result from the first SELECT
	// will be stored in the `orphans` variable, which is later used
	// for recursion in the second SELECT.
	//
	// Then entire recursion happens in the second SELECT, after UNION ALL,
	// and the first SELECT is just to set up the `orphans` variable with
	// the first, initial value. Then, the `orphans` variable is recursively
	// updated with values returned from the second SELECT.
	q := `
		WITH RECURSIVE orphans AS (
			SELECT
				hash
				,prevhash
				,merkleroot
				,height
				,processed_at
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
