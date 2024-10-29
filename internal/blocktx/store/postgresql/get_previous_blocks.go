package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) GetPreviousBlocks(ctx context.Context, hash *chainhash.Hash, n int) ([]*blocktx_api.Block, error) {
	q := `
		WITH RECURSIVE prevBlocks AS (
			SELECT
				hash
				,prevhash
				,merkleroot
				,height
				,processed_at
				,status
				,chainwork
				,1 AS n
			FROM blocktx.blocks WHERE hash = $1
			UNION ALL
			SELECT
				b.hash
				,b.prevhash
				,b.merkleroot
				,b.height
				,b.processed_at
				,b.status
				,b.chainwork
				,p.n+1 AS n
			FROM blocktx.blocks b JOIN prevBlocks p ON b.hash = p.prevhash AND p.n < $2
		)
		SELECT
			hash
		 ,prevhash
		 ,merkleroot
		 ,height
		 ,processed_at
		 ,status
		 ,chainwork
		FROM prevBlocks
	`

	rows, err := p.db.QueryContext(ctx, q, hash[:], n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return p.parseBlocks(rows)
}
