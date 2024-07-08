package validator

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/libsv/go-bt/v2"
)

// getUnminedAncestors returns unmined ancestors with data necessary to perform Deep Fee validation
func GetUnminedAncestors(ctx context.Context, w TxFetcherI, tx *bt.Tx) (map[string]*bt.Tx, error) {
	unmindedAncestorsSet := make(map[string]*bt.Tx)

	prevTxIDs := make([]string, 0, len(tx.Inputs))
	inputsMap := make(map[string][]*bt.Input)

	for _, in := range tx.Inputs {
		id := in.PreviousTxIDStr()
		prevTxIDs = append(prevTxIDs, id)

		inputs, found := inputsMap[id]
		if !found {
			inputs = make([]*bt.Input, 0)
		}

		inputs = append(inputs, in)
		inputsMap[id] = inputs
	}

	parents, err := w.GetRawTxs(ctx, prevTxIDs)
	if err != nil {
		return nil, err
	}

	for _, pTx := range parents {
		if _, found := unmindedAncestorsSet[pTx.TxID]; found {
			continue // parent was proccesed already
		}

		// fulfill data about the parent for furhter validation
		bt, err := bt.NewTxFromString(pTx.Hex)
		if err != nil {
			return nil, err
		}

		for _, input := range inputsMap[pTx.TxID] {
			input.PreviousTxSatoshis = bt.Outputs[input.PreviousTxOutIndex].Satoshis
		}

		if pTx.IsMined {
			continue // we don't need its ancestors
		}

		// get parent ancestors
		parentAncestorsSet, err := GetUnminedAncestors(ctx, w, bt)
		if err != nil {
			return nil, err
		}

		for aID, aTx := range parentAncestorsSet {
			unmindedAncestorsSet[aID] = aTx
		}
	}

	return unmindedAncestorsSet, nil
}

type TxFetcherI interface {
	GetRawTxs(ctx context.Context, ids []string) ([]*RawTx, error)
}

type RawTx struct {
	TxID    string
	Hex     string
	IsMined bool
}

type WocTxFinder struct {
	w       *woc_client.WocClient
	mainnet bool
}

func NewTxFinder() *WocTxFinder {
	// TODO: change/refactor in scope of the ARCO-147
	return &WocTxFinder{
		w:       woc_client.New(),
		mainnet: true,
	}
}

func (w *WocTxFinder) GetRawTxs(ctx context.Context, ids []string) ([]*RawTx, error) {
	wocTxs, err := w.w.GetRawTxs(ctx, w.mainnet, ids)
	if err != nil {
		return nil, fmt.Errorf("getting transaction from WoC for Deep Fees Validation failed. Reason: %w", err)
	}

	res := make([]*RawTx, len(wocTxs))
	for i, tx := range wocTxs {
		res[i] = &RawTx{
			TxID:    tx.TxID,
			Hex:     tx.Hex,
			IsMined: tx.BlockHeight > 0,
		}
	}

	return res, nil
}
