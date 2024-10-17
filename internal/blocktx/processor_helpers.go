package blocktx

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/libsv/go-p2p"
)

func createBlock(msg *p2p.BlockMessage, prevBlock *blocktx_api.Block, longestTipExists bool) *blocktx_api.Block {
	hash := msg.Header.BlockHash()
	prevHash := msg.Header.PrevBlock
	merkleRoot := msg.Header.MerkleRoot
	chainwork := calculateChainwork(msg.Header.Bits)

	var status blocktx_api.Status
	if prevBlock == nil {
		if longestTipExists {
			status = blocktx_api.Status_ORPHANED
		} else {
			status = blocktx_api.Status_LONGEST
		}
	} else {
		status = prevBlock.Status
	}

	return &blocktx_api.Block{
		Hash:         hash[:],
		PreviousHash: prevHash[:],
		MerkleRoot:   merkleRoot[:],
		Height:       msg.Height,
		Status:       status,
		Chainwork:    chainwork.String(),
	}
}

func getLowestHeight(blocks []*blocktx_api.Block) uint64 {
	if len(blocks) == 0 {
		return 0
	}

	lowest := blocks[0].Height
	for _, b := range blocks {
		if b.Height < lowest {
			lowest = b.Height
		}
	}

	return lowest
}

// calculateChainwork calculates chainwork from the given difficulty bits
//
// This function comes from block-header-service:
// https://github.com/bitcoin-sv/block-headers-service/blob/baa6f2a526f93f611eaf9ff9eb94356a50547ad5/domains/chainwork.go#L6
func calculateChainwork(bits uint32) *big.Int {
	// Return a work value of zero if the passed difficulty bits represent
	// a negative number. Note this should not happen in practice with valid
	// blocks, but an invalid block could trigger it.
	difficultyNum := compactToBig(bits)
	if difficultyNum.Sign() <= 0 {
		return big.NewInt(0)
	}
	// (1 << 256) / (difficultyNum + 1)
	denominator := new(big.Int).Add(difficultyNum, big.NewInt(1))
	// oneLsh256 is 1 shifted left 256 bits.
	oneLsh256 := new(big.Int).Lsh(big.NewInt(1), 256)
	return new(big.Int).Div(oneLsh256, denominator)
}

// compactToBig  takes a compact representation of a 256-bit number used in Bitcoin,
// converts it to a big.Int, and returns the resulting big.Int value.
//
// This function comes from block-header-service:
// https://github.com/bitcoin-sv/block-headers-service/blob/baa6f2a526f93f611eaf9ff9eb94356a50547ad5/domains/chainwork.go#L72
func compactToBig(compact uint32) *big.Int {
	// Extract the mantissa, sign bit, and exponent.
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes to represent the full 256-bit number.  So,
	// treat the exponent as the number of bytes and shift the mantissa
	// right or left accordingly.  This is equivalent to:
	// N = mantissa * 256^(exponent-3)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}

	// Make it negative if the sign bit is set.
	if isNegative {
		bn = bn.Neg(bn)
	}

	return bn
}

func progressIndices(total, steps int) map[int]int {
	totalF := float64(total)
	stepsF := float64(steps)

	step := int(math.Max(math.Round(totalF/stepsF), 1))
	stepF := float64(step)

	progress := make(map[int]int)
	for i := float64(1); i < stepsF; i++ {
		progress[step*int(i)] = int(stepF * i / totalF * 100)
	}

	progress[total] = 100
	return progress
}

func withRetry(ctx context.Context, fn func() error, n int) error {
	n = int(math.Max(float64(n), 1)) // ensure n is at least 1
	var rerr error

	for i := range n {
		select {
		case <-ctx.Done():
			rerr = errors.Join(rerr, fmt.Errorf("failure on %d try: %w", i+1, ctx.Err()))
			return rerr

		default:
			err := fn()

			// return if the function completes successfully
			if err == nil {
				return nil
			}

			rerr = errors.Join(rerr, fmt.Errorf("failure on %d try: %w", i+1, err))
			time.Sleep(100 * time.Millisecond)
		}
	}

	return rerr
}
