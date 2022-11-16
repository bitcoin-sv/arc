package defaultvalidator

import (
	"fmt"

	"github.com/TAAL-GmbH/mapi/validator"
	"github.com/bitcoinsv/bsvd/txscript"
	"github.com/bitcoinsv/bsvutil"
	"github.com/libsv/go-bt/v2"
)

type DefaultValidator struct {
}

func New() validator.Validator {
	return &DefaultValidator{}
}

func (v *DefaultValidator) ValidateTransaction(tx *bt.Tx, parentData map[validator.Outpoint]validator.OutpointData) error {
	if err := checkFees(tx); err != nil {
		return err
	}

	if err := checkScripts(tx.Bytes(), parentData); err != nil {
		return err
	}

	return nil
}

func checkFees(tx *bt.Tx) error {
	return nil
}

func checkScripts(txBytes []byte, parentData map[validator.Outpoint]validator.OutpointData) error {
	// Convert the go-bt transaction in to a bsvd.Msg
	tmp, err := bsvutil.NewTxFromBytes(txBytes)
	if err != nil {
		return fmt.Errorf("Could not read tx bytes: %w", err)
	}

	tx := tmp.MsgTx()

	sigHashes := txscript.NewTxSigHashes(tx)

	flags := txscript.ScriptBip16 |
		txscript.ScriptVerifyDERSignatures |
		txscript.ScriptStrictMultiSig |
		txscript.ScriptDiscourageUpgradableNops |
		txscript.ScriptVerifyBip143SigHash

	for i, in := range tx.TxIn {

		outpoint := validator.Outpoint{
			Txid: in.PreviousOutPoint.Hash.String(),
			Idx:  in.PreviousOutPoint.Index,
		}

		outpointData, found := parentData[outpoint]
		if !found {
			return fmt.Errorf("Outpoint %#v not found", outpoint)
		}

		vm, err := txscript.NewEngine(outpointData.ScriptPubKey, tx, i, flags, nil, sigHashes, outpointData.Satoshis)
		if err != nil {
			return fmt.Errorf("Could not create VM: %w", err)
		}

		if err := vm.Execute(); err != nil {
			if err.Error() != "script returned early" {
				return fmt.Errorf("Script execution failed: %w", err)
			}
		}
	}

	return nil
}
