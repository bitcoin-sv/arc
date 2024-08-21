package broadcaster

import (
	"github.com/bitcoin-sv/arc/internal/fees"
	primitives "github.com/bitcoin-sv/go-sdk/primitives/ec"
	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/transaction/template/p2pkh"
)

func PayTo(tx *transaction.Transaction, script *script.Script, satoshis uint64) error {
	if !script.IsP2PKH() {
		return transaction.ErrInvalidScriptType
	}

	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      satoshis,
		LockingScript: script,
	})
	return nil
}

func SignAllInputs(tx *transaction.Transaction, privKey *primitives.PrivateKey) error {
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

func CalculateFeeSat(tx *transaction.Transaction, feeModel fees.FeeModel) uint64 {
	size := calculateTxStdSize(tx)
	varIntUpper := transaction.VarInt(tx.OutputCount()).UpperLimitInc()
	if varIntUpper == -1 {
		return 0
	}

	changeOutputFee := varIntUpper
	changeP2pkhByteLen := uint64(8 + 1 + 25)
	totalBytes := size + changeP2pkhByteLen

	miningFeeSat, err := feeModel.ComputeFeeBasedOnSize(totalBytes)
	if err != nil {
		return 0
	}

	txFees := miningFeeSat + uint64(changeOutputFee)

	return txFees
}

func calculateTxStdSize(tx *transaction.Transaction) uint64 {
	dataLen := 0
	for _, d := range tx.Outputs {
		if d.LockingScript.IsData() {
			dataLen += len(*d.LockingScript)
		}
	}
	return uint64(tx.Size() - dataLen)
}