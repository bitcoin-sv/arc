package postgresql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
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

	if err := p.db.QueryRowContext(ctx, q, hash[:]).Scan(
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
