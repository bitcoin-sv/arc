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
	const finderSource = validator.SourceTransactionHandler | validator.SourceNodes | validator.SourceWoC

	parentsTxs, err := w.GetRawTxs(ctx, finderSource, parentsIDs)
	if err != nil {
		return fmt.Errorf("failed to get raw transactions for parent: %v. Reason: %w", parentsIDs, err)
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

// getUnminedAncestors returns unmined ancestors with data necessary to perform Deep Fee validation
func getUnminedAncestors(ctx context.Context, w validator.TxFinderI, tx *bt.Tx) (map[string]*bt.Tx, error) {
	unmindedAncestorsSet := make(map[string]*bt.Tx)

	// get distinct parents
	// map parentID with inputs collection to avoid duplication and simplify later processing
	parentInputMap := make(map[string][]*bt.Input)
	parentsIDs := make([]string, 0, len(tx.Inputs))

	for _, in := range tx.Inputs {
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
	const finderSource = validator.SourceTransactionHandler | validator.SourceWoC
	parentsTxs, err := w.GetRawTxs(ctx, finderSource, parentsIDs)
	if err != nil {
		return nil, fmt.Errorf("cannot extend tx: %w", err)
	}

	if len(parentsTxs) != len(parentsIDs) {
		return nil, errParentNotFound
	}

	for _, p := range parentsTxs {
		if _, found := unmindedAncestorsSet[p.TxID]; found {
			continue // parent was proccesed already
		}

		childInputs, found := parentInputMap[p.TxID]
		if !found {
			return nil, errParentNotFound
		}

		// fulfill data about the parent for furhter validation
		err := extendInputs(p.Bytes, childInputs)
		if err != nil {
			return nil, err
		}

		if p.IsMined {
			continue // we don't need its ancestors
		}

		// TODO: refactorize it
		bt, _ := bt.NewTxFromBytes(p.Bytes)

		// get parent ancestors
		parentAncestorsSet, err := getUnminedAncestors(ctx, w, bt)
		if err != nil {
			return nil, err
		}

		for aID, aTx := range parentAncestorsSet {
			unmindedAncestorsSet[aID] = aTx
		}
	}

	return unmindedAncestorsSet, nil
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
