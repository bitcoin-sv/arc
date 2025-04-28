package postgresql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bsv-blockchain/go-sdk/util"
)

func (p *PostgreSQL) VerifyMerkleRoots(
	_ context.Context,
	merkleRoots []*blocktx_api.MerkleRootVerificationRequest,
	maxAllowedBlockHeightMismatch uint64,
) (*blocktx_api.MerkleRootVerificationResponse, error) {
	qTopHeight := `
		SELECT MAX(b.height), MIN(b.height) FROM blocktx.blocks b WHERE b.is_longest = true AND b.processed_at IS NOT NULL
	`

	var topHeight uint64
	var lowestHeight uint64

	err := p.db.QueryRow(qTopHeight).Scan(&topHeight, &lowestHeight)
	if errors.Is(err, sql.ErrNoRows) {
		err = store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	qMerkleRoot := `
		SELECT b.height FROM blocktx.blocks b WHERE b.merkleroot = $1 AND b.height = $2 AND b.is_longest = true AND b.processed_at IS NOT NULL
	`

	var unverifiedBlockHeights []uint64

	for _, mr := range merkleRoots {
		merkleBytes, err := hex.DecodeString(mr.MerkleRoot)
		if err != nil {
			unverifiedBlockHeights = append(unverifiedBlockHeights, mr.BlockHeight)
			continue
		}

		merkleBytes = util.ReverseBytes(merkleBytes)

		if err = p.db.QueryRow(qMerkleRoot, merkleBytes, mr.BlockHeight).Scan(new(interface{})); err != nil {
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

func isWithinAllowedMismatch(blockHeight, topHeight uint64, maxMismatch uint64) bool {
	return blockHeight > topHeight && blockHeight-topHeight <= maxMismatch
}
