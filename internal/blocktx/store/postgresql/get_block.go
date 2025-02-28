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
	predicate := "WHERE hash = $1"

	return p.queryBlockByPredicate(ctx, predicate, hash[:])
}

func (p *PostgreSQL) GetLongestBlockByHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error) {
	predicate := "WHERE height = $1 AND is_longest = true"

	return p.queryBlockByPredicate(ctx, predicate, height)
}

func (p *PostgreSQL) GetChainTip(ctx context.Context, heightRange int) (*blocktx_api.Block, error) {
	predicate := `WHERE height = (SELECT MAX(height) from blocktx.blocks WHERE is_longest = true)
									AND is_longest = true
									AND height > (SELECT MAX(height) - $1 from blocktx.blocks)
									AND processed_at IS NOT NULL
	`

	return p.queryBlockByPredicate(ctx, predicate, heightRange)
}

func (p *PostgreSQL) queryBlockByPredicate(ctx context.Context, predicate string, predicateParams ...any) (*blocktx_api.Block, error) {
	q := `
		SELECT
			hash
		 ,prevhash
		 ,merkleroot
		 ,height
		 ,processed_at
		 ,status
		 ,chainwork
		FROM blocktx.blocks
	`

	q += " " + predicate + " AND processed_at IS NOT NULL"

	var block blocktx_api.Block

	var processedAt sql.NullString

	if err := p.db.QueryRowContext(ctx, q, predicateParams...).Scan(
		&block.Hash,
		&block.PreviousHash,
		&block.MerkleRoot,
		&block.Height,
		&processedAt,
		&block.Status,
		&block.Chainwork,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrBlockNotFound
		}
		return nil, err
	}

	block.Processed = processedAt.Valid

	return &block, nil
}
