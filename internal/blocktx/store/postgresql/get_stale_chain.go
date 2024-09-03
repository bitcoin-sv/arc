package postgresql

import (
	"context"
	"database/sql"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) GetStaleChainBackFromHash(ctx context.Context, hash *chainhash.Hash) ([]*blocktx_api.Block, error) {
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

	rows, err := p.db.QueryContext(ctx, q, hash[:], blocktx_api.Status_STALE)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var block blocktx_api.Block
		var processed_at sql.NullString

		err := rows.Scan(
			&block.Hash,
			&block.PreviousHash,
			&block.MerkleRoot,
			&block.Height,
			&processed_at,
			&block.Orphaned,
			&block.Status,
			&block.Chainwork,
		)
		if err != nil {
			return nil, err
		}

		block.Processed = processed_at.Valid

		staleBlocks = append(staleBlocks, &block)
	}

	return staleBlocks, nil
}
