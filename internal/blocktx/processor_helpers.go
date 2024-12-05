package blocktx

import (
	"math"
	"math/big"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func getHashStringNoErr(hash []byte) string {
	chash, err := chainhash.NewHash(hash)
	if err != nil {
		return ""
	}
	return chash.String()
}

func sumChainwork(blocks []*blocktx_api.Block) *big.Int {
	sum := big.NewInt(0)
	for _, b := range blocks {
		chainwork := new(big.Int)
		chainwork.SetString(b.Chainwork, 10)

		sum = sum.Add(sum, chainwork)
	}

	return sum
}

func exclusiveRightTxs(leftTxs, rightTxs []store.TransactionBlock) []store.TransactionBlock {
	leftTxsMap := make(map[string]struct{})

	for _, tx := range leftTxs {
		leftTxsMap[string(tx.TxHash)] = struct{}{}
	}

	exclusiveRightTxs := make([]store.TransactionBlock, 0)
	for _, tx := range rightTxs {
		if _, found := leftTxsMap[string(tx.TxHash)]; !found {
			exclusiveRightTxs = append(exclusiveRightTxs, tx)
		}
	}

	return exclusiveRightTxs
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
