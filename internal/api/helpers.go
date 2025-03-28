package api

import (
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
)

func FeesToFeeModel(minMiningFee float64) *feemodel.SatoshisPerKilobyte {
	satoshisPerKB := int(minMiningFee * 1e8)
	return &feemodel.SatoshisPerKilobyte{Satoshis: uint64(satoshisPerKB)}
}
