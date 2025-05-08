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
func (p *PostgreSQL) UnorphanRecentWrongOrphans(ctx context.Context) (healedOrphans []*blocktx_api.Block, err error) {
	// The way this query works, is that the result from the first SELECT
	// will be stored in the `recent_orphans` variable, which is later used
	// for recursion in the second SELECT.
	//
	// Then entire recursion happens in the second SELECT, after UNION ALL,
	// and the first SELECT is just to set up the `recent_orphans` variable with
	// the first, initial value. Then, the `recent_orphans` variable is recursively
	// updated with values returned from the second SELECT.
	q_drop_temp_table := `drop table if exists tmp_blocks_to_unorphan;`
	q_create_temp_table := `
create table blocks_to_unorphan AS
SELECT btu.hash, btu.prevhash, btu.status FROM (
    WITH RECURSIVE recent_orphans AS (SELECT *
                                        FROM (SELECT hash
                                                   , prevhash
                                                   , status
                                              FROM blocktx.blocks
                                              ORDER BY height DESC
                                              LIMIT 1000) recent_longest
                                        WHERE recent_longest.status = $1
                                        UNION ALL
                                        SELECT child.hash
                                             , child.prevhash
                                             , child.status
                                        FROM recent_orphans as parent
                                                 JOIN blocktx.blocks as child ON child.prevhash = parent.hash
                                        WHERE child.processed_at IS NOT NULL
                                          AND child.status = $2)
      SELECT hash
           , prevhash
           , status
      FROM recent_orphans
      WHERE status = $2
) as btu;
`
	q_update_unorphan_blocks := `
	UPDATE blocktx.blocks b
	SET status = $1
	FROM blocks_to_unorphan
	WHERE blocks_to_unorphan.hash = b.hash;
`

	q_select_updated_blocks := `
	SELECT b.hash
		, b.prevhash
		, b.merkleroot
		, b.height
		, b.processed_at
		, b.status
		, b.chainwork from blocktx.blocks b JOIN blocks_to_unorphan btu on b.hash = btu.hash
	WHERE b.status = $1
	ORDER BY height ASC;
`
	p.db.Begin()
	_, err = p.db.ExecContext(ctx, q_drop_temp_table)
	if err != nil {
		return
	}
	_, err = p.db.ExecContext(ctx, q_create_temp_table, blocktx_api.Status_LONGEST, blocktx_api.Status_ORPHANED)
	if err != nil {
		return
	}
	_, err = p.db.ExecContext(ctx, q_update_unorphan_blocks, blocktx_api.Status_LONGEST)
	if err != nil {
		return
	}
	rows, err := p.db.QueryContext(ctx, q_select_updated_blocks, blocktx_api.Status_LONGEST)
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
