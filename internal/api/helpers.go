package api

import (
	"github.com/bitcoin-sv/arc/internal/fees"
)

func FeesToFeeModel(minMiningFee float64) *fees.SatoshisPerKilobyte {
	satoshisPerKB := int(minMiningFee * 1e8)
	return &fees.SatoshisPerKilobyte{Satoshis: uint64(satoshisPerKB)}
}
