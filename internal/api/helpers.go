package api

import (
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
	"github.com/ccoveille/go-safecast"
)

func FeesToFeeModel(minMiningFee float64) *feemodel.SatoshisPerKilobyte {
	satoshisPerKB := int(minMiningFee * 1e8)
	satoshisPerKBuint64, err := safecast.ToUint64(satoshisPerKB)
	if err != nil {
		return nil
	}
	return &feemodel.SatoshisPerKilobyte{Satoshis: satoshisPerKBuint64}
}
