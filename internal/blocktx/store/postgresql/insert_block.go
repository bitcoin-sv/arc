package postgresql

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"go.opentelemetry.io/otel/trace"
)

func (p *PostgreSQL) InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "InsertBlock")
		defer span.End()
	}

	qInsert := `
		INSERT INTO blocktx.blocks (hash, prevhash, merkleroot, height, status, chainwork)
		VALUES ($1 ,$2 , $3, $4, $5, $6)
		ON CONFLICT (hash) DO UPDATE SET orphanedyn = FALSE
		RETURNING id
	`

	var blockId uint64

	row := p.db.QueryRowContext(ctx, qInsert,
		block.GetHash(),
		block.GetPreviousHash(),
		block.GetMerkleRoot(),
		block.GetHeight(),
		block.GetStatus(),
		block.GetChainwork(),
	)

	err := row.Scan(&blockId)
	if err != nil {
		return 0, fmt.Errorf("failed when inserting block: %v", err)
	}

	return blockId, nil
}
