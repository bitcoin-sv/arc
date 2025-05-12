package postgresql

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

// UnorphanRecentWrongOrphans searches for the LONGEST blocks in the last 1000
// then recursively searches for blocks marked as ORPHANED
// whose hash matches the prevhash of the given block,
// and then repeats that recursively for each newly found orphaned block until
// it has the entire orphaned chain.
func (p *PostgreSQL) UnorphanRecentWrongOrphans(ctx context.Context) (unorphanedBlocks []*blocktx_api.Block, err error) {
	// The way this query works, is that the result from the first SELECT
	// will be stored in the `recent_orphans` variable, which is later used
	// for recursion in the second SELECT.
	//
	// Then entire recursion happens in the second SELECT, after UNION ALL,
	// and the first SELECT is just to set up the `recent_orphans` variable with
	// the first, initial value. Then, the `recent_orphans` variable is recursively
	// updated with values returned from the second SELECT.

	qUnorphanBlocks := `
	WITH updated AS (
		UPDATE blocktx.blocks
		SET status = $1,
		is_longest = true
		WHERE hash IN
		( WITH RECURSIVE recent_orphans AS (SELECT *
	
																  FROM (SELECT hash
																			 , status
																			 , prevhash
																			 , processed_at
																		FROM blocktx.blocks
																		ORDER BY height DESC
																		LIMIT 1000) recent_longest
																  WHERE recent_longest.status = $1
																  UNION ALL
																  SELECT child.hash
																	   , child.status
																	   , child.prevhash
																	   , child.processed_at
																  FROM recent_orphans as parent
																		   JOIN blocktx.blocks as child ON child.prevhash = parent.hash
																  WHERE child.processed_at IS NOT NULL
																	AND child.status = $2
																    AND (SELECT COUNT(*) /* does not have a sibling that is LONGEST */ 
																         FROM blocktx.blocks c
																         WHERE c.prevhash = parent.hash
																         AND c.processed_at IS NOT NULL
																         AND c.status = $1
																         ) = 0
																	 AND (SELECT COUNT(*) /* does not have a sibling that is ORPHAN but with higher chainwork */
																	 FROM blocktx.blocks c
																	 WHERE c.prevhash = parent.hash
																	 AND c.hash != child.hash
																	 AND c.status = $2
																	 AND c.processed_at IS NOT NULL
																	 AND c.chainwork > child.chainwork
																	 ) = 0
																    )
								SELECT  r.hash
								FROM recent_orphans r
								WHERE status = $2
								)
	RETURNING  hash
		, prevhash
		, merkleroot
		, height
		, processed_at
		, status
		, chainwork
)
SELECT *
FROM updated
ORDER BY height ASC;
`
	rows, err := p.db.QueryContext(ctx, qUnorphanBlocks, blocktx_api.Status_LONGEST, blocktx_api.Status_ORPHANED)
	if err != nil {
		return
	}
	defer rows.Close()
	unorphanedBlocks, err = p.parseBlocks(rows)
	if err != nil {
		return
	}

	return
}
