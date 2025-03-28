package defaultvalidator

import (
	"context"
	"errors"
	"fmt"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

var (
	ErrParentNotFound              = errors.New("parent transaction not found")
	ErrFailedToGetMempoolAncestors = errors.New("failed to get mempool ancestors from finder")
)

func extendTx(ctx context.Context, txFinder validator.TxFinderI, rawTx *sdkTx.Transaction, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) (err error) {
	ctx, span := tracing.StartTracing(ctx, "extendTx", tracingEnabled, tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// potential improvement: implement version for the rawTx with only one input

	// get distinct parents
	parentsIDs := make([]string, 0, len(rawTx.Inputs))

	// map parentID with inputs collection to avoid duplication and simplify later processing
	parentInputMap := make(map[string][]*sdkTx.TransactionInput)

	for _, in := range rawTx.Inputs {
		prevTxID := in.SourceTXID.String()

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

	parentsTxs := txFinder.GetRawTxs(ctx, finderSource, parentsIDs)
	if len(parentsTxs) != len(parentsIDs) {
		return ErrParentNotFound
	}

	// extend inputs with parents data
	for _, p := range parentsTxs {
		childInputs, found := parentInputMap[p.TxID().String()]
		if !found {
			return ErrParentNotFound
		}

		if err = extendInputs(p, childInputs); err != nil {
			return err
		}
	}

	return nil
}

func extendInputs(tx *sdkTx.Transaction, childInputs []*sdkTx.TransactionInput) error {
	for _, input := range childInputs {
		if len(tx.Outputs) < int(input.SourceTxOutIndex) {
			return fmt.Errorf("output %d not found in transaction %s", input.SourceTxOutIndex, input.SourceTXID.String())
		}
		output := tx.Outputs[input.SourceTxOutIndex]

		input.SetSourceTxOutput(output)
	}

	return nil
}

// getUnminedAncestors returns unmined ancestors with data necessary to perform cumulative fee validation
func getUnminedAncestors(ctx context.Context, txFinder validator.TxFinderI, tx *sdkTx.Transaction, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) (unmindedAncestorsSet map[string]*sdkTx.Transaction, err error) {
	ctx, span := tracing.StartTracing(ctx, "getUnminedAncestors", tracingEnabled, tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()
	unmindedAncestorsSet = make(map[string]*sdkTx.Transaction)

	// get distinct parents
	parentsIDs := make([]string, 0, len(tx.Inputs))
	parentInputMap := make(map[string]struct{})

	for _, in := range tx.Inputs {
		prevTxID := in.SourceTXID.String()
		_, found := parentInputMap[prevTxID]
		if !found {
			// first occurrence of the parent
			parentsIDs = append(parentsIDs, prevTxID)
		}
	}

	mempoolAncestorTxIDs, err := txFinder.GetMempoolAncestors(ctx, parentsIDs)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetMempoolAncestors, err)
	}

	var allMempoolTxs []*sdkTx.Transaction
	if len(mempoolAncestorTxIDs) > 0 {
		const finderSource = validator.SourceTransactionHandler | validator.SourceNodes | validator.SourceWoC
		allMempoolTxs = txFinder.GetRawTxs(ctx, finderSource, mempoolAncestorTxIDs)
	}

	for _, mempoolTx := range allMempoolTxs {
		err = extendTx(ctx, txFinder, mempoolTx, tracingEnabled, tracingAttributes...)
		if err != nil {
			return nil, err
		}

		unmindedAncestorsSet[mempoolTx.TxID().String()] = mempoolTx
	}

	return unmindedAncestorsSet, nil
}
