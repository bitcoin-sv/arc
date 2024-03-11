package postgresql

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/ordishs/gocore"
)

func (p *PostgreSQL) InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("InsertBlock").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	qInsert := `
		INSERT INTO blocks (hash, prevhash, merkleroot, height)
		VALUES ($1 ,$2 , $3, $4)
		ON CONFLICT (hash) DO UPDATE SET orphanedyn = FALSE
		RETURNING id
	`

	var blockId uint64

	err := p.db.QueryRowContext(ctx, qInsert, block.GetHash(), block.GetPreviousHash(), block.GetMerkleRoot(), block.GetHeight()).Scan(&blockId)
	if err != nil {
		return 0, fmt.Errorf("failed when inserting block: %v", err)
	}

	return blockId, nil
}
