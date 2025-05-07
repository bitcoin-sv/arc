package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

// AutoHealOrphans searches for the LONGEST blocks in the last 1000
// then recursively searches for blocks marked as ORPHANED
// whose hash matches the prevhash of the given block,
// and then repeats that recursively for each newly found orphaned block until
// it has the entire orphaned chain.
func (p *PostgreSQL) AutoHealOrphans(ctx context.Context) (healedOrphans []*blocktx_api.Block, err error) {
	// The way this query works, is that the result from the first SELECT
	// will be stored in the `recent_orphans` variable, which is later used
	// for recursion in the second SELECT.
	//
	// Then entire recursion happens in the second SELECT, after UNION ALL,
	// and the first SELECT is just to set up the `recent_orphans` variable with
	// the first, initial value. Then, the `recent_orphans` variable is recursively
	// updated with values returned from the second SELECT.
	q := `
		WITH RECURSIVE recent_orphans AS (
			SELECT 	* FROM (SELECT 	hash
									,prevhash
									,merkleroot
									,height
									,processed_at
									,status
									,chainwork
			                FROM blocktx.blocks
							ORDER BY height DESC
							LIMIT 1000
			) recent_longest
			WHERE recent_longest.status = $1
			UNION ALL
			SELECT child.hash
				,child.prevhash
				,child.merkleroot
				,child.height
				,child.processed_at
				,child.status
				,child.chainwork 
			FROM recent_orphans as parent
		   JOIN blocktx.blocks as child ON child.prevhash = parent.hash
			WHERE child.processed_at IS NOT NULL
			AND child.status = $2
		)
		SELECT 	hash
				,prevhash
				,merkleroot
				,height
				,processed_at
				,status
				,chainwork
		FROM recent_orphans
		WHERE status = $2
		ORDER BY height
	`

	rows, err := p.db.QueryContext(ctx, q, blocktx_api.Status_LONGEST, blocktx_api.Status_ORPHANED)
	if err != nil {
		return
	}
	defer rows.Close()

	healedOrphans, err = p.parseBlocks(rows)
	if err != nil {
		return
	}

	return
}
