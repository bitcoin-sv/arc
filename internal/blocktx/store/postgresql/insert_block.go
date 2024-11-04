package postgresql

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

func (p *PostgreSQL) UpsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	ctx, span := tracing.StartTracing(ctx, "UpsertBlock", p.tracingEnabled, p.tracingAttributes...)
	defer tracing.EndTracing(span)

	qInsert := `
		INSERT INTO blocktx.blocks (hash, prevhash, merkleroot, height, status, chainwork)
		VALUES ($1 ,$2 , $3, $4, $5, $6)
		ON CONFLICT (hash) DO UPDATE SET orphanedyn = FALSE
		RETURNING id
	`

	var blockID uint64

	row := p.db.QueryRowContext(ctx, qInsert,
		block.GetHash(),
		block.GetPreviousHash(),
		block.GetMerkleRoot(),
		block.GetHeight(),
		block.GetStatus(),
		block.GetChainwork(),
	)

	err := row.Scan(&blockID)
	if err != nil {
		return 0, errors.Join(store.ErrFailedToInsertBlock, err)
	}

	return blockID, nil
}
