package sql

import (
	"context"
	"database/sql"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/gocore"
	"github.com/pkg/errors"
)

func (s *SQL) GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetBlock").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.prevhash
		,b.merkleroot
		,b.height
		,b.processed_at
		,b.orphanedyn
		FROM blocks b
		WHERE b.hash = $1
	`

	var block blocktx_api.Block

	var processed_at sql.NullString

	if err := s.db.QueryRowContext(ctx, q, hash[:]).Scan(
		&block.Hash,
		&block.PreviousHash,
		&block.MerkleRoot,
		&block.Height,
		&processed_at,
		&block.Orphaned,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrBlockNotFound
		}
		return nil, err
	}

	block.Processed = processed_at.Valid

	return &block, nil
}
