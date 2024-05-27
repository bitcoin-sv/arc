package beef

import (
	"errors"
	"fmt"

	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
)

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

	if !parent.Unmined() {
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
