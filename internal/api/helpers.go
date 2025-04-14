package api

import (
	"fmt"
	"math"

	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
)

func FeesToFeeModel(minMiningFee float64) *feemodel.SatoshisPerKilobyte {
	satoshisPerKB := int(minMiningFee * 1e8)
	satoshisPerKBuint64, err := SafeIntToUint64(satoshisPerKB)
	if err != nil {
		return nil
	}
	return &feemodel.SatoshisPerKilobyte{Satoshis: satoshisPerKBuint64}
}

func SafeIntToUint64(i int) (uint64, error) {
	if i < 0 {
		return 0, fmt.Errorf("negative value cannot be converted to uint64: %d", i)
	}
	return uint64(i), nil
}

func SafeInt64ToUint64(i int64) (uint64, error) {
	if i < 0 {
		return 0, fmt.Errorf("negative value cannot be converted to uint64: %d", i)
	}
	return uint64(i), nil
}

func SafeInt64ToInt(u uint64) (int, error) {
	if u > uint64(math.MaxInt) {
		return 0, fmt.Errorf("value too large for int: %d", u)
	}
	return int(u), nil
}

func SafeIntToUint32(i int) (uint32, error) {
	if i < 0 {
		return 0, fmt.Errorf("negative value cannot be converted to uint32: %d", i)
	}
	if i > math.MaxUint32 {
		return 0, fmt.Errorf("value too large for uint32: %d", i)
	}
	return uint32(i), nil
}
