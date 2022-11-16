package gobtvalidator

import (
	"fmt"

	"github.com/TAAL-GmbH/mapi/validator"
	"github.com/libsv/go-bt/v2"

	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/bscript/interpreter"
	_ "github.com/libsv/go-bt/v2/bscript/interpreter"
)

type GoBtValidator struct{}

func New() validator.Validator {
	return &GoBtValidator{}
}

func (v *GoBtValidator) ValidateTransaction(tx *bt.Tx, parentData map[validator.Outpoint]validator.OutpointData) error {
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
	tx, err := bt.NewTxFromBytes(txBytes)
	if err != nil {
		return err
	}
	for i, in := range tx.Inputs {
		outpoint := validator.Outpoint{
			Txid: in.PreviousTxIDStr(),
			Idx:  in.PreviousTxOutIndex,
		}

		outpointData, found := parentData[outpoint]
		if !found {
			return fmt.Errorf("Outpoint %#v not found", outpoint)
		}

		prevOutput := &bt.Output{
			Satoshis:      uint64(outpointData.Satoshis),
			LockingScript: (*bscript.Script)(&outpointData.ScriptPubKey),
		}

		if err := interpreter.NewEngine().Execute(
			interpreter.WithTx(tx, i, prevOutput),
			interpreter.WithForkID(),
			interpreter.WithAfterGenesis(),
		); err != nil {
			// if err.Error() != "script returned early" {
			return fmt.Errorf("Script execution failed: %w", err)
			// }

		}
	}

	return nil

}
