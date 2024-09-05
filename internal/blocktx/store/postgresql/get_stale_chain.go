package postgresql

import (
	"context"
	"database/sql"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

// GetStaleChainBackFromHash is a function that recursively searches for blocks
// marked as STALE from the given hash - back to the block marked as LONGEST,
// which is the common ancestor for the STALE and LONGEST chains.
//
// It searches for the block by given hash and finds parent blocks recursively
// using the prevhash field from that found block.
//
// A 		In this scenario, the block A, B and D are marked as LONGEST
// |\ 		while blocks C and E are marked as STALE.
// B C
// | | 		Function GetStaleChainBackFromHash(ctx, E), given the hash E
// D E 		will return blocks C and E, which is the entire STALE chain.
func (p *PostgreSQL) GetStaleChainBackFromHash(ctx context.Context, hash []byte) ([]*blocktx_api.Block, error) {
	// The way this query works, is that the result from the first SELECT
	// will be stored in the prevBlocks variable, which is later used
	// for recursion in the second SELECT.
	//
	// Then entire recursion happens in the second SELECT, after UNION ALL,
	// and the first SELECT is just to set up the prevBlocks variable with
	// the first, initial value. Then, the prevBlocks variable is recursively
	// updated with values returned from the second SELECT.
	q := `
		WITH RECURSIVE prevBlocks AS (
			SELECT
				hash
				,prevhash
				,merkleroot
				,height
				,processed_at
				,orphanedyn
				,status
				,chainwork
			FROM blocktx.blocks WHERE hash = $1
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
			FROM blocktx.blocks b JOIN prevBlocks p ON b.hash = p.prevhash AND b.status = $2
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
		FROM prevBlocks
	`
	staleBlocks := make([]*blocktx_api.Block, 0)

	rows, err := p.db.QueryContext(ctx, q, hash, blocktx_api.Status_STALE)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var block blocktx_api.Block
		var processedAt sql.NullString

		err := rows.Scan(
			&block.Hash,
			&block.PreviousHash,
			&block.MerkleRoot,
			&block.Height,
			&processedAt,
			&block.Orphaned,
			&block.Status,
			&block.Chainwork,
		)
		if err != nil {
			return nil, err
		}

		block.Processed = processedAt.Valid

		staleBlocks = append(staleBlocks, &block)
	}

	return staleBlocks, nil
}
