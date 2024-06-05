package beef

import (
	"errors"
	"fmt"

	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript/interpreter"
)

func CalculateInputsOutputsSatoshis(tx *bt.Tx, inputTxs []*TxData) (uint64, uint64, error) {
	inputSum := uint64(0)

	for _, input := range tx.Inputs {
		inputParentTx := findParentForInput(input, inputTxs)

		if inputParentTx == nil {
			return 0, 0, errors.New("invalid parent transactions, no matching trasactions for input")
		}

		inputSum += inputParentTx.Transaction.Outputs[input.PreviousTxOutIndex].Satoshis
	}

	outputSum := tx.TotalOutputSatoshis()

	return inputSum, outputSum, nil
}

func ValidateScripts(tx *bt.Tx, inputTxs []*TxData) error {
	for i, input := range tx.Inputs {
		inputParentTx := findParentForInput(input, inputTxs)
		if inputParentTx == nil {
			return errors.New("invalid parent transactions, no matching trasactions for input")
		}

		err := verifyScripts(tx, inputParentTx.Transaction, i)
		if err != nil {
			return errors.New("invalid script")
		}
	}

	return nil
}

func verifyScripts(tx, prevTx *bt.Tx, inputIdx int) error {
	input := tx.InputIdx(inputIdx)
	prevOutput := prevTx.OutputIdx(int(input.PreviousTxOutIndex))

	err := interpreter.NewEngine().Execute(
		interpreter.WithTx(tx, inputIdx, prevOutput),
		interpreter.WithForkID(),
		interpreter.WithAfterGenesis(),
	)

	return err
}

func EnsureAncestorsArePresentInBump(tx *bt.Tx, beefTx *BEEF) error {
	minedAncestors := make([]*TxData, 0)

	for _, input := range tx.Inputs {
		if err := findMinedAncestorsForInput(input, beefTx.Transactions, &minedAncestors); err != nil {
			return err
		}
	}

	for _, tx := range minedAncestors {
		if !existsInBumps(tx, beefTx.BUMPs) {
			return errors.New("invalid BUMP - input mined ancestor is not present in BUMPs")
		}
	}

	return nil
}

func findMinedAncestorsForInput(input *bt.Input, ancestors []*TxData, minedAncestors *[]*TxData) error {
	parent := findParentForInput(input, ancestors)
	if parent == nil {
		return fmt.Errorf("invalid BUMP - cannot find mined parent for input %s", input.String())
	}

	if parent.IsMined() {
		*minedAncestors = append(*minedAncestors, parent)
		return nil
	}

	for _, in := range parent.Transaction.Inputs {
		err := findMinedAncestorsForInput(in, ancestors, minedAncestors)
		if err != nil {
			return err
		}
	}

	return nil
}

func findParentForInput(input *bt.Input, parentTxs []*TxData) *TxData {
	parentID := input.PreviousTxIDStr()

	for _, ptx := range parentTxs {
		if ptx.GetTxID() == parentID {
			return ptx
		}
	}

	return nil
}

func existsInBumps(tx *TxData, bumps []*bc.BUMP) bool {
	bumpIdx := int(*tx.BumpIndex)
	txID := tx.GetTxID()

	if len(bumps) > bumpIdx {
		leafs := bumps[bumpIdx].Path[0]

		for _, lf := range leafs {
			if txID == *lf.Hash {
				return true
			}
		}
	}

	return false
}
