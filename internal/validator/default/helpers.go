package defaultvalidator

import (
	"context"
	"errors"
	"fmt"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/validator"
)

var (
	ErrParentNotFound              = errors.New("parent transaction not found")
	ErrFailedToGetRawTxs           = errors.New("failed to get raw transactions for parent")
	ErrFailedToGetMempoolAncestors = errors.New("failed to get mempool ancestors")
)

func extendTx(ctx context.Context, txFinder validator.TxFinderI, rawTx *sdkTx.Transaction, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) error {
	ctx, span := tracing.StartTracing(ctx, "extendTx", tracingEnabled, tracingAttributes...)
	defer tracing.EndTracing(span)

	// potential improvement: implement version for the rawTx with only one input

	// get distinct parents
	// map parentID with inputs collection to avoid duplication and simplify later processing
	parentInputMap := make(map[string][]*sdkTx.TransactionInput)
	parentsIDs := make([]string, 0, len(rawTx.Inputs))

	for _, in := range rawTx.Inputs {
		prevTxID := in.PreviousTxIDStr()

		inputs, found := parentInputMap[prevTxID]
		if !found {
			// first occurrence of the parent
			inputs = make([]*sdkTx.TransactionInput, 0)
			parentsIDs = append(parentsIDs, prevTxID)
		}

		inputs = append(inputs, in)
		parentInputMap[prevTxID] = inputs
	}

	// get parents
	const finderSource = validator.SourceTransactionHandler | validator.SourceNodes | validator.SourceWoC

	parentsTxs, err := txFinder.GetRawTxs(ctx, finderSource, parentsIDs)
	if err != nil {
		return errors.Join(ErrFailedToGetRawTxs, fmt.Errorf("failed to get raw transactions for parents %v: %w", parentsIDs, err))
	}

	if len(parentsTxs) != len(parentsIDs) {
		return ErrParentNotFound
	}

	// extend inputs with parents data
	for _, p := range parentsTxs {
		childInputs, found := parentInputMap[p.TxID]
		if !found {
			return ErrParentNotFound
		}

		bTx, err := sdkTx.NewTransactionFromBytes(p.Bytes)
		if err != nil {
			return fmt.Errorf("cannot parse parent tx: %w", err)
		}

		if err = extendInputs(bTx, childInputs); err != nil {
			return err
		}
	}

	return nil
}

// getUnminedAncestors returns unmined ancestors with data necessary to perform cumulative fee validation
func getUnminedAncestors(ctx context.Context, txFinder validator.TxFinderI, tx *sdkTx.Transaction, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) (map[string]*sdkTx.Transaction, error) {
	ctx, span := tracing.StartTracing(ctx, "getUnminedAncestors", tracingEnabled, tracingAttributes...)
	defer tracing.EndTracing(span)
	unmindedAncestorsSet := make(map[string]*sdkTx.Transaction)

	// get distinct parents
	// map parentID with inputs collection to avoid duplication and simplify later processing
	parentInputMap := make(map[string][]*sdkTx.TransactionInput)
	parentsIDs := make([]string, 0, len(tx.Inputs))

	for _, in := range tx.Inputs {
		prevTxID := in.PreviousTxIDStr()

		inputs, found := parentInputMap[prevTxID]
		if !found {
			// first occurrence of the parent
			inputs = make([]*sdkTx.TransactionInput, 0)
			parentsIDs = append(parentsIDs, prevTxID)
		}

		inputs = append(inputs, in)
		parentInputMap[prevTxID] = inputs
	}

	parentsTxs, err := txFinder.GetMempoolAncestors(ctx, parentsIDs)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetMempoolAncestors, fmt.Errorf("failed to get mempool ancestors: %w", err))
	}

	for _, p := range parentsTxs {
		if _, found := unmindedAncestorsSet[p.TxID]; found {
			continue // parent was processed already
		}

		childInputs, found := parentInputMap[p.TxID]
		if !found {
			return nil, ErrParentNotFound
		}

		// fill data about the parent for further validation
		bTx, err := sdkTx.NewTransactionFromBytes(p.Bytes)
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
	}

	return unmindedAncestorsSet, nil
}

func extendInputs(tx *sdkTx.Transaction, childInputs []*sdkTx.TransactionInput) error {
	for _, input := range childInputs {
		if len(tx.Outputs) < int(input.SourceTxOutIndex) {
			return fmt.Errorf("output %d not found in transaction %s", input.SourceTxOutIndex, input.PreviousTxIDStr())
		}
		output := tx.Outputs[input.SourceTxOutIndex]

		input.SetPrevTxFromOutput(output)
	}

	return nil
}
