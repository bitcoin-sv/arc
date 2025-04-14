package api

import (
	"fmt"

	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
)

func FeesToFeeModel(minMiningFee float64) *feemodel.SatoshisPerKilobyte {
	satoshisPerKB := int(minMiningFee * 1e8)
	return &feemodel.SatoshisPerKilobyte{Satoshis: uint64(satoshisPerKB)}
}

func SafeUint64(i int) (uint64, error) {
	if i < 0 {
		return 0, fmt.Errorf("negative value cannot be converted to uint64: %d", i)
	}
	return uint64(i), nil
}
