package utils

import (
	"math"

	"github.com/libsv/go-bt/v2"
)

func EstimateFee(satsPerKB, numberOfInputs, numberOfPayingOutputs uint64) uint64 {
	fundingSize := uint64(4)                              // amount: version
	inputs := bt.VarInt(numberOfInputs)                   // var int of inputs
	fundingSize += uint64(len(inputs))                    // input variable length
	fundingSize += numberOfInputs * 150                   // 32 bytes prev_txid, 4 bytes vout, 110 bytes for sigScript + public key, 4 bytes for sequence
	outputs := bt.VarInt(numberOfPayingOutputs)           // var int of inputs
	fundingSize += uint64(len(outputs))                   // output variable length
	fundingSize += numberOfPayingOutputs * uint64(8+1+25) // 8 bytes for satoshi value + 1 byte length of script, (e.g. 76a914cc371eceb7267d5c76b1200ab63e394ec890305388ac)
	fundingSize += uint64(4)                              // sequence number

	feeRate := float64(satsPerKB) / 1000
	fee := float64(fundingSize) * feeRate
	fee = math.Ceil(fee)
	return uint64(fee)
}
