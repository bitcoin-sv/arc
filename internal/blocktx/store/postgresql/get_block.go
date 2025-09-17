package postgresql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (p *PostgreSQL) GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
	predicate := "WHERE hash = $1"

	return p.queryBlockByPredicate(ctx, predicate, hash[:])
}

func (p *PostgreSQL) GetLongestBlockByHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error) {
	predicate := "WHERE height = $1 AND is_longest = true"

	return p.queryBlockByPredicate(ctx, predicate, height)
}

func (p *PostgreSQL) GetChainTip(ctx context.Context) (*blocktx_api.Block, error) {
	predicate := "WHERE height = (SELECT MAX(height) FROM blocktx.blocks blks WHERE blks.is_longest = true AND processed_at IS NOT NULL)"

	return p.queryBlockByPredicate(ctx, predicate)
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
		 ,timestamp
		FROM blocktx.blocks
	`

	q += " " + predicate + " AND processed_at IS NOT NULL"

	var block blocktx_api.Block

	var processedAt sql.NullTime
	var timestamp sql.NullTime

	if err := p.db.QueryRowContext(ctx, q, predicateParams...).Scan(
		&block.Hash,
		&block.PreviousHash,
		&block.MerkleRoot,
		&block.Height,
		&processedAt,
		&block.Status,
		&block.Chainwork,
		&timestamp,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrBlockNotFound
		}
		return nil, err
	}

	block.Processed = processedAt.Valid

	if processedAt.Valid {
		block.ProcessedAt = timestamppb.New(processedAt.Time)
	}

	if timestamp.Valid {
		block.Timestamp = timestamppb.New(timestamp.Time)
	}

	return &block, nil
}
