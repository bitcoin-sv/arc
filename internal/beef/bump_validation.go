package beef

import (
	"errors"
	"fmt"

	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
)

func EnsureAncestorsArePresentInBump(tx *bt.Tx, dBeef *BEEF) error {
	ancestors, err := findMinedAncestors(tx, dBeef.Transactions)
	if err != nil {
		return err
	}

	for _, tx := range ancestors {
		if !existsInBumps(tx, dBeef.BUMPs) {
			return errors.New("invalid BUMP - input mined ancestor is not present in BUMPs")
		}
	}

	return nil
}

func findMinedAncestors(tx *bt.Tx, ancestors []*TxData) (map[string]*TxData, error) {
	am := make(map[string]*TxData)

	for _, input := range tx.Inputs {
		if err := findMinedAncestorsForInput(input, ancestors, am); err != nil {
			return nil, err
		}
	}

	return am, nil
}

func findMinedAncestorsForInput(input *bt.Input, ancestors []*TxData, am map[string]*TxData) error {
	parent := findParentForInput(input, ancestors)
	if parent == nil {
		return fmt.Errorf("invalid BUMP - cannot find mined parent for input %s", input.String())
	}

	if !parent.Unmined() {
		am[parent.GetTxID()] = parent
		return nil
	}

	for _, in := range parent.Transaction.Inputs {
		err := findMinedAncestorsForInput(in, ancestors, am)
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
