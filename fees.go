package mapi

import (
	"github.com/libsv/go-bt/v2"
)

func FeesToBtFeeQuote(fees *[]Fee) *bt.FeeQuote {

	btFeeQuote := bt.NewFeeQuote()
	for _, fee := range *fees {
		btFeeQuote.AddQuote(bt.FeeType(fee.FeeType), &bt.Fee{
			MiningFee: bt.FeeUnit{
				Satoshis: int(fee.MiningFee.Satoshis),
				Bytes:    int(fee.MiningFee.Bytes),
			},
			RelayFee: bt.FeeUnit{
				Satoshis: int(fee.RelayFee.Satoshis),
				Bytes:    int(fee.RelayFee.Bytes),
			},
		})
	}

	return btFeeQuote
}
