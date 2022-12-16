package sql

import (
	pb "github.com/TAAL-GmbH/arc/blocktx/api"

	"context"
	"database/sql"
)

func (s *SQL) InsertBlock(ctx context.Context, block *pb.Block) (uint64, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		INSERT INTO blocks (
		 hash
		,header
		,height
		) VALUES (
		 $1
		,$2
		,$3
		)
		ON CONFLICT DO NOTHING
		RETURNING id
	`

	var blockId uint64

	if err := s.db.QueryRowContext(ctx, q, block.Hash, block.Header, block.Height).Scan(&blockId); err != nil {
		if err == sql.ErrNoRows {
			// The insert failed because the block already exists.
			// We will mark the block as unorphaned whilst retrieving the id.
			q = `
				UPDATE blocks SET
				 orphanedyn = false
				WHERE hash = $1
				RETURNING id
			`

			if err := s.db.QueryRowContext(ctx, q, block.Hash).Scan(&blockId); err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}

	return blockId, nil
}
