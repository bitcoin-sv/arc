package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) GetBlockGaps(ctx context.Context, blockHeightRange int) ([]*store.BlockGap, error) {

	q := `
			SELECT DISTINCT all_missing.height, all_missing.hash FROM
			(SELECT missing_blocks.missing_block_height AS height, blocktx.blocks.prevhash AS hash FROM blocktx.blocks
				JOIN (
				SELECT bl.block_heights AS missing_block_height FROM (
				SELECT unnest(ARRAY(
					SELECT a.n
					FROM generate_series((SELECT max(height) - $1 AS block_height FROM blocktx.blocks b), (SELECT max(height) AS block_height FROM blocktx.blocks b)) AS a(n)
				)) AS block_heights) AS bl
				LEFT JOIN blocktx.blocks blks ON blks.height = bl.block_heights
				WHERE blks.height IS NULL
				) AS missing_blocks ON blocktx.blocks.height = missing_blocks.missing_block_height + 1
			UNION
			SELECT height, hash FROM blocktx.blocks WHERE processed_at IS NULL AND height < (SELECT max(height) AS block_height FROM blocktx.blocks b)
			AND height > (SELECT max(height) - $1 AS block_height FROM blocktx.blocks b)
			) AS all_missing
			LEFT JOIN blocktx.block_processing bp ON bp.block_hash = all_missing.hash
			WHERE bp IS NULL ORDER BY all_missing.height  DESC;
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

		txHash, err := chainhash.NewHash(hash)
		if err != nil {
			return nil, err
		}

		blockGaps = append(blockGaps, &store.BlockGap{Height: height, Hash: txHash})
	}

	return blockGaps, nil
}
