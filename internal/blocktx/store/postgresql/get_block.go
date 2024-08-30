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

func (p *PostgreSQL) GetBlockByHeight(ctx context.Context, height uint64, status blocktx_api.Status) (*blocktx_api.Block, error) {
	predicate := "WHERE height = $1 AND status = $2"

	return p.queryBlockByPredicate(ctx, predicate, height, status)
}

func (p *PostgreSQL) GetChainTip(ctx context.Context) (*blocktx_api.Block, error) {
	predicate := "WHERE height = (SELECT MAX(height) FROM blocktx.blocks blks WHERE blks.status = $1)"

	return p.queryBlockByPredicate(ctx, predicate, blocktx_api.Status_LONGEST)
}

func (p *PostgreSQL) queryBlockByPredicate(ctx context.Context, predicate string, predicateParams ...any) (*blocktx_api.Block, error) {
	q := `
		SELECT
			hash
		 ,prevhash
		 ,merkleroot
		 ,height
		 ,processed_at
		 ,orphanedyn
		 ,status
		 ,chainwork
		FROM blocktx.blocks
	`

	q += " " + predicate

	var block blocktx_api.Block

	var processed_at sql.NullString

	if err := p.db.QueryRowContext(ctx, q, predicateParams...).Scan(
		&block.Hash,
		&block.PreviousHash,
		&block.MerkleRoot,
		&block.Height,
		&processed_at,
		&block.Orphaned,
		&block.Status,
		&block.Chainwork,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrBlockNotFound
		}
		return nil, err
	}

	block.Processed = processed_at.Valid

	return &block, nil
}
