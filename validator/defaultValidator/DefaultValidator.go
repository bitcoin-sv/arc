package defaultvalidator

import (
	"fmt"

	"github.com/TAAL-GmbH/mapi/validator"
	"github.com/libsv/go-bt/v2"

	"github.com/libsv/go-bt/v2/bscript/interpreter"
	_ "github.com/libsv/go-bt/v2/bscript/interpreter"
)

type DefaultValidator struct{}

func New() validator.Validator {
	return &DefaultValidator{}
}

func (v *DefaultValidator) ValidateTransaction(tx *bt.Tx) error {
	if err := checkFees(tx); err != nil {
		return err
	}

	if err := checkScripts(tx); err != nil {
		return err
	}

	return nil
}

func checkFees(tx *bt.Tx) error {
	return nil
}

func checkScripts(tx *bt.Tx) error {

	for i, in := range tx.Inputs {

		prevOutput := &bt.Output{
			Satoshis:      in.PreviousTxSatoshis,
			LockingScript: in.PreviousTxScript,
		}

		if err := interpreter.NewEngine().Execute(
			interpreter.WithTx(tx, i, prevOutput),
			interpreter.WithForkID(),
			interpreter.WithAfterGenesis(),
		); err != nil {
			return fmt.Errorf("Script execution failed: %w", err)
		}
	}

	return nil

}
