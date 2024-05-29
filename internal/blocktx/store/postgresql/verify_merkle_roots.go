package postgresql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
)

func (p *PostgreSQL) VerifyMerkleRoots(ctx context.Context, merkleRoots []*blocktx_api.MerkleRootVerificationRequest) (*blocktx_api.MerkleRootVerificationResponse, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	qTopHeight := `
		SELECT MAX(b.height) FROM blocks b WHERE b.orphanedyn = false
	`

	var topHeight int64

	err := p.db.QueryRowContext(ctx, qTopHeight).Scan(&topHeight)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	qMerkleRoot := `
		SELECT b.height FROM blocks b WHERE b.merkleroot = $1 AND b.orphanedyn = false
	`

	var unverifiedBlockHeights []uint64

	for _, mr := range merkleRoots {
		var hash []byte

		err := p.db.QueryRowContext(ctx, qMerkleRoot, mr.MerkleRoot).Scan(&hash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				unverifiedBlockHeights = append(unverifiedBlockHeights, mr.BlockHeight)
				continue
			}
			return nil, err
		}
	}

	return &blocktx_api.MerkleRootVerificationResponse{UnverifiedBlockHeights: unverifiedBlockHeights}, nil
}
