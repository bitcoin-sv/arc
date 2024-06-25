package postgresql

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"go.opentelemetry.io/otel/trace"
)

func (p *PostgreSQL) InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "InsertBlock")
		defer span.End()
	}

	qInsert := `
		INSERT INTO blocktx.blocks (hash, prevhash, merkleroot, height)
		VALUES ($1 ,$2 , $3, $4)
		ON CONFLICT (hash) DO UPDATE SET orphanedyn = FALSE
		RETURNING id
	`

	var blockId uint64
	fmt.Println(block.GetHash())
	fmt.Println(block.GetPreviousHash())
	fmt.Println(block.GetMerkleRoot())
	fmt.Println(block.GetHeight())
	err := p.db.QueryRowContext(ctx, qInsert, block.GetHash(), block.GetPreviousHash(), block.GetMerkleRoot(), block.GetHeight()).Scan(&blockId)
	if err != nil {
		return 0, fmt.Errorf("failed when inserting block: %v", err)
	}

	return blockId, nil
}
