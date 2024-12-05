package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) GetBlockGaps(ctx context.Context, blockHeightRange int) ([]*store.BlockGap, error) {
	// Flow of this query:
	//
	// 1. Get height - 1 and prevhash from blocks where there isn't a previous block
	// and where height is greater than our height range parameter.
	//
	// 2. Add to result from 1. all blocks from the blocks table that are unprocessed yet.
	//
	// 3. Combine the result from 1. and 2. with block_processing table and remove all
	// results that are being currently processed.
	//
	// 4. Sort by height.
	q := `
		SELECT DISTINCT all_missing.missing_height, all_missing.missing_hash
		FROM (
				SELECT b.height - 1 AS missing_height, b.prevhash AS missing_hash
				FROM blocktx.blocks b
				WHERE b.height > (SELECT max(height) - $1 FROM blocktx.blocks)
						AND NOT EXISTS (SELECT 1 FROM blocktx.blocks missing WHERE missing.hash = b.prevhash)
				UNION
				SELECT unprocessed.height AS missing_height, unprocessed.hash AS missing_hash
				FROM blocktx.blocks unprocessed
				WHERE unprocessed.processed_at IS NULL
						AND unprocessed.height > (SELECT max(height) - $1 FROM blocktx.blocks)
		) AS all_missing
		LEFT JOIN blocktx.block_processing bp ON bp.block_hash = all_missing.missing_hash
		WHERE bp.block_hash IS NULL
		ORDER BY all_missing.missing_height ASC;
	`

	rows, err := p.db.QueryContext(ctx, q, blockHeightRange)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blockGaps := make([]*store.BlockGap, 0)
	for rows.Next() {
		var height uint64
		var hash []byte
		err = rows.Scan(&height, &hash)
		if err != nil {
			return nil, err
		}

		// in e2e tests, peers will misbehave if we ask
		// for a genesis block, so we need to ignore it
		if height == uint64(0) {
			continue
		}

		txHash, err := chainhash.NewHash(hash)
		if err != nil {
			return nil, err
		}

		blockGaps = append(blockGaps, &store.BlockGap{Height: height, Hash: txHash})
	}

	return blockGaps, nil
}
