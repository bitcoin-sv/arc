package postgresql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
)

func (p *PostgreSQL) VerifyMerkleRoots(
	ctx context.Context,
	merkleRoots []*blocktx_api.MerkleRootVerificationRequest,
	maxAllowedBlockHeightMismatch int,
) (*blocktx_api.MerkleRootVerificationResponse, error) {
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
		SELECT b.height FROM blocks b WHERE b.merkleroot = $1 AND b.height = $2 AND b.orphanedyn = false
	`

	var unverifiedBlockHeights []uint64

	for _, mr := range merkleRoots {
		var hash []byte

		merkleBytes, err := hex.DecodeString(mr.MerkleRoot)
		if err != nil {
			unverifiedBlockHeights = append(unverifiedBlockHeights, mr.BlockHeight)
			continue
		}

		merkleBytes = reverseBytes(merkleBytes)
		// merkleBytesQueryStr := fmt.Sprintf("0x%x", merkleBytes)

		err = p.db.QueryRowContext(ctx, qMerkleRoot, merkleBytes, mr.BlockHeight).Scan(&hash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if !isWithinAllowedMismatch(mr.BlockHeight, topHeight, maxAllowedBlockHeightMismatch) {
					unverifiedBlockHeights = append(unverifiedBlockHeights, mr.BlockHeight)
				}
				continue
			}
			return nil, err
		}
	}

	return &blocktx_api.MerkleRootVerificationResponse{UnverifiedBlockHeights: unverifiedBlockHeights}, nil
}

func reverseBytes(b []byte) []byte {
	n := len(b)
	reversed := make([]byte, n)
	for i := 0; i < n; i++ {
		reversed[i] = b[n-1-i]
	}
	return reversed
}

func isWithinAllowedMismatch(blockHeight uint64, topHeight int64, maxMismatch int) bool {
	return blockHeight > uint64(topHeight) && blockHeight-uint64(topHeight) <= uint64(maxMismatch)
}
