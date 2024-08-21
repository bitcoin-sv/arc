package defaultvalidator

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/go-sdk/transaction"
)

var errParentNotFound = errors.New("parent transaction not found")

func extendTx(ctx context.Context, w validator.TxFinderI, rawTx *transaction.Transaction) error {
	// potential improvement: implement version for the rawTx with only one input

	// get distinct parents
	// map parentID with inputs collection to avoid duplication and simplify later processing
	parentInputMap := make(map[string][]*transaction.TransactionInput)
	parentsIDs := make([]string, 0, len(rawTx.Inputs))

	for _, in := range rawTx.Inputs {
		prevTxID := in.PreviousTxIDStr()

		inputs, found := parentInputMap[prevTxID]
		if !found {
			// first occurrence of the parent
			inputs = make([]*transaction.TransactionInput, 0)
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

	// extend inputs with parents data
	for _, p := range parentsTxs {
		childInputs, found := parentInputMap[p.TxID]
		if !found {
			return errParentNotFound
		}

		bTx, err := transaction.NewTransactionFromBytes(p.Bytes)
		if err != nil {
			return fmt.Errorf("cannot parse parent tx: %w", err)
		}

		if err = extendInputs(bTx, childInputs); err != nil {
			return err
		}
	}

	return nil
}

// getUnminedAncestors returns unmined ancestors with data necessary to perform Deep Fee validation
func getUnminedAncestors(ctx context.Context, w validator.TxFinderI, tx *transaction.Transaction) (map[string]*transaction.Transaction, error) {
	unmindedAncestorsSet := make(map[string]*transaction.Transaction)

	// get distinct parents
	// map parentID with inputs collection to avoid duplication and simplify later processing
	parentInputMap := make(map[string][]*transaction.TransactionInput)
	parentsIDs := make([]string, 0, len(tx.Inputs))

	for _, in := range tx.Inputs {
		prevTxID := in.PreviousTxIDStr()

		inputs, found := parentInputMap[prevTxID]
		if !found {
			// first occurrence of the parent
			inputs = make([]*transaction.TransactionInput, 0)
			parentsIDs = append(parentsIDs, prevTxID)
		}

		inputs = append(inputs, in)
		parentInputMap[prevTxID] = inputs
	}

	// get parents
	const finderSource = validator.SourceTransactionHandler | validator.SourceWoC
	parentsTxs, err := w.GetRawTxs(ctx, finderSource, parentsIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get raw transactions for parent: %v. Reason: %w", parentsIDs, err)
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

		// fulfill data about the parent for further validation
		bTx, err := transaction.NewTransactionFromBytes(p.Bytes)
		if err != nil {
			return nil, fmt.Errorf("cannot parse parent tx: %w", err)
		}

		err = extendInputs(bTx, childInputs)
		if err != nil {
			return nil, err
		}

		if p.IsMined {
			continue // we don't need its ancestors
		}

		unmindedAncestorsSet[p.TxID] = bTx

		// get parent ancestors
		parentAncestorsSet, err := getUnminedAncestors(ctx, w, bTx)
		if err != nil {
			return nil, err
		}

		for aID, aTx := range parentAncestorsSet {
			unmindedAncestorsSet[aID] = aTx
		}
	}

	return unmindedAncestorsSet, nil
}

func extendInputs(tx *transaction.Transaction, childInputs []*transaction.TransactionInput) error {
	for _, input := range childInputs {
		if len(tx.Outputs) < int(input.SourceTxOutIndex) {
			return fmt.Errorf("output %d not found in transaction %s", input.SourceTxOutIndex, input.PreviousTxIDStr())
		}
		output := tx.Outputs[input.SourceTxOutIndex]

		input.SetPrevTxFromOutput(output)
	}

	return nil
}
