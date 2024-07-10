package defaultvalidator

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/libsv/go-bt/v2"
)

var errParentNotFound = errors.New("parent transaction not found")

func extendTx(ctx context.Context, w validator.TxFinderI, rawTx *bt.Tx) error {
	// potential improvement: implement version for the rawTx with only one input

	// get distinct parents
	// map parentID with inputs collection to avoid duplication and simplify later processing
	parentInputMap := make(map[string][]*bt.Input)
	parentsIDs := make([]string, 0, len(rawTx.Inputs))

	for _, in := range rawTx.Inputs {
		prevTxID := in.PreviousTxIDStr()

		inputs, found := parentInputMap[prevTxID]
		if !found {
			// first occurrence of the parent
			inputs = make([]*bt.Input, 0)
			parentsIDs = append(parentsIDs, prevTxID)
		}

		inputs = append(inputs, in)
		parentInputMap[prevTxID] = inputs
	}

	// get parents
	parentsTxs, err := w.GetRawTxs(ctx, parentsIDs)
	if err != nil {
		return fmt.Errorf("cannot extend tx: %w", err)
	}

	if len(parentsTxs) != len(parentsIDs) {
		return errParentNotFound
	}

	// extend inputs with partents data
	for _, p := range parentsTxs {
		childInputs, found := parentInputMap[p.TxID]
		if !found {
			return errParentNotFound
		}

		if err = extendInputs(p.Bytes, childInputs); err != nil {
			return err
		}
	}

	return nil
}

func extendInputs(prevTx []byte, childInputs []*bt.Input) error {
	tx, err := bt.NewTxFromBytes(prevTx)
	if err != nil {
		return fmt.Errorf("cannot parse parent tx: %w", err)
	}

	for _, input := range childInputs {
		if len(tx.Outputs) < int(input.PreviousTxOutIndex) {
			return fmt.Errorf("output %d not found in transaction %s", input.PreviousTxOutIndex, input.PreviousTxIDStr())
		}
		output := tx.Outputs[input.PreviousTxOutIndex]

		input.PreviousTxScript = output.LockingScript
		input.PreviousTxSatoshis = output.Satoshis
	}

	return nil
}
