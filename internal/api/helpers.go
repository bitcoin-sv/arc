package api

import (
	"github.com/bitcoin-sv/bdk/module/gobdk/script"
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
)

func FeesToFeeModel(minMiningFee float64) *feemodel.SatoshisPerKilobyte {
	satoshisPerKB := uint64(minMiningFee * 1e8)
	return &feemodel.SatoshisPerKilobyte{Satoshis: satoshisPerKB}
}

type ScriptVerifier interface {
	VerifyScript(extendedTX []byte, utxoHeights []int32, blockHeight int32, consensus bool) script.ScriptError
}
