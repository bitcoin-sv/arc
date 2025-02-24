package fees

import (
	"math"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
)

type FeeModel interface {
	ComputeFee(tx *sdkTx.Transaction) (uint64, error)
	ComputeFeeBasedOnSize(txSize uint64) (uint64, error)
}

type SatoshisPerKilobyte struct {
	Satoshis uint64
}

// ComputeFee calculates the transaction fee based on its size in bytes.
// The fee is computed as the transaction size (in kilobytes) multiplied by the
// satoshis per kilobyte rate.
//
// This method was originally copied from the Go-SDK library.
//
// Change Description:
// Previously, the fee calculation used `math.Ceil` to round up the transaction size
// to the nearest kilobyte, which resulted in charging the full fee for a kilobyte
// even for transactions smaller than 1000 bytes. For example, a transaction with a
// size of 500 bytes would be charged the same fee as a 1000-byte transaction.
//
// The updated implementation removes the rounding, so the fee is now directly
// proportional to the transaction size. For instance, a transaction with 500 bytes
// will now be charged 50% of the fee for a full kilobyte, ensuring a fairer and
// more accurate fee calculation.
func (s SatoshisPerKilobyte) ComputeFee(tx *sdkTx.Transaction) (uint64, error) {
	txSize := tx.Size()

	feesRequiredRounded := computeFee(uint64(txSize), s)

	return feesRequiredRounded, nil
}

// ComputeFeeBasedOnSize calculates the transaction fee based on the transaction size in bytes.
func (s SatoshisPerKilobyte) ComputeFeeBasedOnSize(txSize uint64) (uint64, error) {
	feesRequiredRounded := computeFee(txSize, s)

	return feesRequiredRounded, nil
}

func DefaultSatoshisPerKilobyte() SatoshisPerKilobyte {
	return SatoshisPerKilobyte{Satoshis: 1}
}

func computeFee(txSize uint64, s SatoshisPerKilobyte) uint64 {
	fee := float64(txSize) * float64(s.Satoshis) / 1000

	// the minimum fees required is 1 satoshi
	feesRequiredRounded := uint64(math.Ceil(fee))
	if feesRequiredRounded < 1 {
		feesRequiredRounded = 1
	}
	return feesRequiredRounded
}
