package main

import "github.com/libsv/go-bt/v2"

var feeQuote *bt.FeeQuote

func init() {
	feeQuote = bt.NewFeeQuote()

	feeQuote.AddQuote(bt.FeeTypeStandard, &bt.Fee{
		FeeType: bt.FeeTypeStandard,
		MiningFee: bt.FeeUnit{
			Satoshis: 50,
			Bytes:    1000,
		},
		RelayFee: bt.FeeUnit{
			Satoshis: 0,
			Bytes:    1000,
		},
	})

	feeQuote.AddQuote(bt.FeeTypeData, &bt.Fee{
		FeeType: bt.FeeTypeData,
		MiningFee: bt.FeeUnit{
			Satoshis: 50,
			Bytes:    1000,
		},
		RelayFee: bt.FeeUnit{
			Satoshis: 0,
			Bytes:    1000,
		},
	})
}
