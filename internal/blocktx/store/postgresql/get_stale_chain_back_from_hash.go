package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) GetStaleChainBackFromHash(ctx context.Context, hash *chainhash.Hash) ([]*blocktx_api.Block, error) {
	q := `
		WITH RECURSIVE prevBlocks AS (
			SELECT * FROM blocktx.blocks WHERE hash = $1
			UNION ALL
			SELECT * FROM blocktx.blocks b JOIN prevBlocks p ON b.hash = p.prevhash
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
	return nil, nil
}
