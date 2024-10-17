package broadcaster

import (
	primitives "github.com/bitcoin-sv/go-sdk/primitives/ec"
	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/transaction/template/p2pkh"
)

func PayTo(tx *sdkTx.Transaction, script *script.Script, satoshis uint64) error {
	if !script.IsP2PKH() {
		return sdkTx.ErrInvalidScriptType
	}

	tx.AddOutput(&sdkTx.TransactionOutput{
		Satoshis:      satoshis,
		LockingScript: script,
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
