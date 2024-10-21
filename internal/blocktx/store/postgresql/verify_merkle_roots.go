package postgresql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/go-sdk/util"
)

func (p *PostgreSQL) VerifyMerkleRoots(
	_ context.Context,
	merkleRoots []*blocktx_api.MerkleRootVerificationRequest,
	maxAllowedBlockHeightMismatch int,
) (*blocktx_api.MerkleRootVerificationResponse, error) {
	qTopHeight := `
		SELECT MAX(b.height), MIN(b.height) FROM blocktx.blocks b WHERE b.status != $1
	`

	var topHeight uint64
	var lowestHeight uint64

	err := p.db.QueryRow(qTopHeight, blocktx_api.Status_ORPHANED).Scan(&topHeight, &lowestHeight)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	qMerkleRoot := `
		SELECT b.height FROM blocktx.blocks b WHERE b.merkleroot = $1 AND b.height = $2 AND b.status != $3
	`

	var unverifiedBlockHeights []uint64

	for _, mr := range merkleRoots {
		merkleBytes, err := hex.DecodeString(mr.MerkleRoot)
		if err != nil {
			unverifiedBlockHeights = append(unverifiedBlockHeights, mr.BlockHeight)
			continue
		}

		merkleBytes = util.ReverseBytes(merkleBytes)

		err = p.db.QueryRow(qMerkleRoot, merkleBytes, mr.BlockHeight, blocktx_api.Status_ORPHANED).Scan(new(interface{}))
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
