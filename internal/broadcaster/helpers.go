package broadcaster

import (
	"math"

	primitives "github.com/bitcoin-sv/go-sdk/primitives/ec"
	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/transaction/template/p2pkh"
)

func PayTo(tx *sdkTx.Transaction, s *script.Script, satoshis uint64) error {
	if !s.IsP2PKH() {
		return sdkTx.ErrInvalidScriptType
	}

	tx.AddOutput(&sdkTx.TransactionOutput{
		Satoshis:      satoshis,
		LockingScript: s,
	})
	return nil
}

func SignAllInputs(tx *sdkTx.Transaction, privKey *primitives.PrivateKey) error {
	unlockingScriptTemplate, err := p2pkh.Unlock(privKey, nil)
	if err != nil {
		return err
	}

	for _, in := range tx.Inputs {
		if in.UnlockingScriptTemplate != nil {
			continue
		}

		in.UnlockingScriptTemplate = unlockingScriptTemplate
	}

	err = tx.Sign()
	if err != nil {
		return err
	}
	return nil
}

func EstimateSize(tx *sdkTx.Transaction) int {
	size := 4                                             // version
	size += sdkTx.VarInt(uint64(len(tx.Inputs))).Length() // number of inputs

	inputSize := len(tx.Inputs) * (40 + 1 + 107) // txid, output index, sequence number, script length
	size += inputSize

	size += sdkTx.VarInt(len(tx.Outputs)).Length() // number of outputs
	for _, out := range tx.Outputs {
		size += 8 // satoshis
		length := len(*out.LockingScript)
		size += sdkTx.VarInt(length).Length() // script length
		size += length                        // script
	}
	size += 4  // lock time
	size += 34 // change

	return size
}

type FeeModel interface {
	ComputeFee(tx *sdkTx.Transaction) (uint64, error)
	ComputeFeeBasedOnSize(txSize uint64) (uint64, error)
}

type SatoshisPerKilobyte struct {
	Satoshis uint64
}

// ComputeFee calculates the transaction fee based on its size in bytes.
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
	feesRequiredRounded := uint64(math.Round(fee))
	if feesRequiredRounded < 1 {
		feesRequiredRounded = 1
	}
	return feesRequiredRounded
}
