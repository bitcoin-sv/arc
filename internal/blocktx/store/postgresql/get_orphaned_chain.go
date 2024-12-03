package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

// GetOrphansBackToNonOrphanAncestor recursively searches for blocks marked
// as ORPHANED from the given hash - back to the first ORPHANED block. Then, it
// tries to get the first non-orphaned ancestor of that orphan chain.
//
// It searches for the block whose hash matches the prevhash of the given block,
// and then repeats that recursively for each newly found orphaned block until
// it has the entire orphaned chain.
func (p *PostgreSQL) GetOrphansBackToNonOrphanAncestor(ctx context.Context, hash []byte) (orphans []*blocktx_api.Block, nonOrphanAncestor *blocktx_api.Block, err error) {
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
			FROM blocktx.blocks WHERE hash = $1 AND status = $2
			UNION ALL
			SELECT
				b.hash
				,b.prevhash
				,b.merkleroot
				,b.height
				,b.processed_at
				,b.status
				,b.chainwork
			FROM blocktx.blocks b JOIN orphans o ON o.prevhash = b.hash AND b.status = $2
			WHERE b.processed_at IS NOT NULL
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
		ORDER BY height
	`

	rows, err := p.db.QueryContext(ctx, q, hash, blocktx_api.Status_ORPHANED)
	if err != nil {
		return
	}
	defer rows.Close()

	orphans, err = p.parseBlocks(rows)
	if err != nil {
		return
	}

	// first element in orphans
	// will be the given block
	if len(orphans) < 2 {
		return
	}

	// try to get first non-orphan ancestor
	nonOrphanHash, err := chainhash.NewHash(orphans[0].PreviousHash)
	if err != nil {
		return
	}

	nonOrphanAncestor, _ = p.GetBlock(ctx, nonOrphanHash)

	return
}
