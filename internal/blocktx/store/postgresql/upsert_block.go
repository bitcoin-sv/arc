package postgresql

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

func (p *PostgreSQL) UpsertBlock(ctx context.Context, block *blocktx_api.Block) (blockID uint64, err error) {
	ctx, span := tracing.StartTracing(ctx, "UpsertBlock", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// This query will insert a block ONLY if one of the 3 conditions is met:
	// 1. Block being inserted is `ORPHANED` or `LONGEST` and there's no previous block in the database
	// 2. The block being inserted has the same status as its previous block
	// 3. The block being inserted has status `STALE` but the previous block was `LONGEST`
	// Any other situation would mean an error in block processing
	// (probably because of another block which is being inserted by another blocktx instance at the same time)
	// and requires the block to be received and processed again.
	qInsert := `
		INSERT INTO blocktx.blocks (hash, prevhash, merkleroot, height, status, chainwork, is_longest)
		SELECT v.hash, v.prevhash, v.merkleroot, v.height, v.status, v.chainwork, v.is_longest
		FROM (VALUES ($1::BYTEA, $2::BYTEA, $3::BYTEA, $4::BIGINT, $5::INTEGER, $6::TEXT, $7::BOOLEAN))
				AS v(hash, prevhash, merkleroot, height, status, chainwork, is_longest)
		LEFT JOIN blocktx.blocks AS prevblock ON prevblock.hash = v.prevhash
		WHERE ((v.status = $8 OR v.status = $9) AND prevblock.id IS NULL)
				OR prevblock.status = $5
				OR (prevblock.status = $9 AND $5 = $10)
		ON CONFLICT (hash) DO UPDATE SET status = EXCLUDED.status
		RETURNING id
	`

	row := p.db.QueryRowContext(ctx, qInsert,
		block.GetHash(),
		block.GetPreviousHash(),
		block.GetMerkleRoot(),
		block.GetHeight(),
		block.GetStatus(),
		block.GetChainwork(),
		block.GetStatus() == blocktx_api.Status_LONGEST,
		blocktx_api.Status_ORPHANED,
		blocktx_api.Status_LONGEST,
		blocktx_api.Status_STALE,
	)

	err = row.Scan(&blockID)
	if err != nil {
		return 0, errors.Join(store.ErrFailedToInsertBlock, err)
	}

	return blockID, nil
}

//func (p *PostgreSQL) UpdateLongest(ctx context.Context, blockHash *chainhash.Hash) (err error) {
//
//	return nil
//}
