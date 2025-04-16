package api

import (
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
)

func FeesToFeeModel(minMiningFee float64) *feemodel.SatoshisPerKilobyte {
	satoshisPerKB := uint64(minMiningFee * 1e8)
	return &feemodel.SatoshisPerKilobyte{Satoshis: satoshisPerKB}
}
