package postgresql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-bt/v2"
)

func (p *PostgreSQL) VerifyMerkleRoots(
	_ context.Context,
	merkleRoots []*blocktx_api.MerkleRootVerificationRequest,
	maxAllowedBlockHeightMismatch int,
) (*blocktx_api.MerkleRootVerificationResponse, error) {
	qTopHeight := `
		SELECT MAX(b.height), MIN(b.height) FROM blocktx.blocks b WHERE b.orphanedyn = false
	`

	var topHeight uint64
	var lowestHeight uint64

	err := p.db.QueryRow(qTopHeight).Scan(&topHeight, &lowestHeight)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	qMerkleRoot := `
		SELECT b.height FROM blocktx.blocks b WHERE b.merkleroot = $1 AND b.height = $2 AND b.orphanedyn = false
	`

	var unverifiedBlockHeights []uint64

	for _, mr := range merkleRoots {
		merkleBytes, err := hex.DecodeString(mr.MerkleRoot)
		if err != nil {
			unverifiedBlockHeights = append(unverifiedBlockHeights, mr.BlockHeight)
			continue
		}

		merkleBytes = bt.ReverseBytes(merkleBytes)

		err = p.db.QueryRow(qMerkleRoot, merkleBytes, mr.BlockHeight).Scan(new(interface{}))
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}

			if isOlderThanLowestHeight(mr.BlockHeight, lowestHeight) {
				continue
			}

			if isWithinAllowedMismatch(mr.BlockHeight, topHeight, maxAllowedBlockHeightMismatch) {
				continue
			}

			unverifiedBlockHeights = append(unverifiedBlockHeights, mr.BlockHeight)
		}
	}

	return &blocktx_api.MerkleRootVerificationResponse{UnverifiedBlockHeights: unverifiedBlockHeights}, nil
}

func isOlderThanLowestHeight(blockHeight, lowestHeight uint64) bool {
	return blockHeight < lowestHeight
}

func isWithinAllowedMismatch(blockHeight, topHeight uint64, maxMismatch int) bool {
	return blockHeight > topHeight && blockHeight-topHeight <= uint64(maxMismatch)
}
