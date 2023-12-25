package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/ordishs/gocore"
)

func (s *SQL) InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("InsertBlock").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		INSERT INTO blocks (
		 hash
		,prevhash
		,merkleroot
		,height
		) VALUES (
		 $1
		,$2
		,$3
		,$4
		)
		ON CONFLICT DO NOTHING
		RETURNING id
	`

	var blockId uint64

	if err := s.db.QueryRowContext(ctx, q, block.GetHash(), block.GetPreviousHash(), block.GetMerkleRoot(), block.GetHeight()).Scan(&blockId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// The insert failed because the block already exists.
			// We will mark the block as un-orphaned whilst retrieving the id.
			q = `
				UPDATE blocks SET
				 orphanedyn = false
				WHERE hash = $1
				RETURNING id
			`

			if err := s.db.QueryRowContext(ctx, q, block.GetHash()).Scan(&blockId); err != nil {
				return 0, fmt.Errorf("failed when updating block: %v", err)
			}
		} else {
			return 0, fmt.Errorf("failed when inserting block: %v", err)
		}
	}

	return blockId, nil
}
