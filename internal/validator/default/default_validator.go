package defaultvalidator

import (
	"context"
	"errors"
	"fmt"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"

	internalApi "github.com/bitcoin-sv/arc/internal/api"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

var (
	ErrTxFeeTooLow = errors.New("transaction fee is too low")
)

type DefaultValidator struct {
	policy   *bitcoin.Settings
	txFinder validator.TxFinderI
}

func New(policy *bitcoin.Settings, finder validator.TxFinderI) *DefaultValidator {
	return &DefaultValidator{
		policy:   policy,
		txFinder: finder,
	}
}

func (v *DefaultValidator) ValidateTransaction(ctx context.Context, tx *sdkTx.Transaction, feeValidation validator.FeeValidation, scriptValidation validator.ScriptValidation, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) error { //nolint:funlen - mostly comments
	var err *validator.Error
	var spanErr error
	ctx, span := tracing.StartTracing(ctx, "ValidateTransaction", tracingEnabled, tracingAttributes...)
	defer func() {
		if err != nil {
			spanErr = err.Err
		}
		tracing.EndTracing(span, spanErr)
	}()

	// 0) Check whether we have a complete transaction in extended format, with all input information
	//    we cannot check the satoshi input, OP_RETURN is allowed 0 satoshis
	if needsExtension(tx, feeValidation, scriptValidation) {
		err := extendTx(ctx, v.txFinder, tx, tracingEnabled, tracingAttributes...)
		if err != nil {
			return validator.NewError(err, api.ErrStatusTxFormat)
		}
	}

	// The rest of the validation steps
	err = validator.CommonValidateTransaction(v.policy, tx)
	if err != nil {
		return err
	}

	// 10) Reject if the sum of input values is less than sum of output values
	// 11) Reject if transaction fee would be too low (minRelayTxFee) to get into an empty block.
	switch feeValidation {
	case validator.StandardFeeValidation:
		f, err := internalApi.FeesToFeeModel(v.policy.MinMiningTxFee)
		if err != nil {
			return err
		}
		if err = checkStandardFees(tx, f); err != nil {
			return err
		}
	case validator.CumulativeFeeValidation:
		txSet, err := getUnminedAncestors(ctx, v.txFinder, tx, tracingEnabled, tracingAttributes...)
		if err != nil {
			e := fmt.Errorf("getting all unmined ancestors for CFV failed. reason: %w. found: %d", err, len(txSet))
			return validator.NewError(e, api.ErrStatusCumulativeFees)
		}
		f, err := internalApi.FeesToFeeModel(v.policy.MinMiningTxFee)
		if err != nil {
			return err
		}
		vErr := checkCumulativeFees(ctx, txSet, tx, f, tracingEnabled, tracingAttributes...)
		if vErr != nil {
			return vErr
		}
	case validator.NoneFeeValidation:
		// Do not handle the default case on purpose; we shouldn't assume that other types of validation should be omitted
	}

	// 12) The unlocking scripts for each input must validate against the corresponding output locking scripts
	if scriptValidation == validator.StandardScriptValidation {
		if err := checkScripts(tx); err != nil {
			return validator.NewError(err, api.ErrStatusUnlockingScripts)
		}
	}

	// everything checks out
	return nil
}
func needsExtension(tx *sdkTx.Transaction, fv validator.FeeValidation, sv validator.ScriptValidation) bool {
	// don't need if we don't validate fee AND scripts
	if fv == validator.NoneFeeValidation && sv == validator.NoneScriptValidation {
		return false
	}

	// don't need if is extended already
	return !isExtended(tx)
}

func isExtended(tx *sdkTx.Transaction) bool {
	if tx == nil || tx.Inputs == nil {
		return false
	}

	for _, input := range tx.Inputs {
		if input.SourceTxScript() == nil || (*input.SourceTxSatoshis() == 0 && !input.SourceTxScript().IsData()) {
			return false
		}
	}

	return true
}

func checkStandardFees(tx *sdkTx.Transaction, feeModel sdkTx.FeeModel) *validator.Error {
	feesOK, expFeesPaid, actualFeePaid, err := isFeePaidEnough(feeModel, tx)
	if err != nil {
		return validator.NewError(err, api.ErrStatusFees)
	}

	if !feesOK {
		err = errors.Join(ErrTxFeeTooLow, fmt.Errorf("minimum expected fee: %d, actual fee: %d", expFeesPaid, actualFeePaid))
		return validator.NewError(err, api.ErrStatusFees)
	}

	return nil
}

func checkCumulativeFees(ctx context.Context, txSet map[string]*sdkTx.Transaction, tx *sdkTx.Transaction, feeModel *feemodel.SatoshisPerKilobyte, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) (vErr *validator.Error) {
	var spanErr error
	_, span := tracing.StartTracing(ctx, "checkCumulativeFees", tracingEnabled, tracingAttributes...)
	defer func() {
		if vErr != nil {
			spanErr = vErr.Err
		}
		tracing.EndTracing(span, spanErr)
	}()

	expectedCumulativeFee := uint64(0)
	cumulativePaidFeeAncestors := uint64(0)

	totalInputTx, err := tx.TotalInputSatoshis()
	if err != nil {
		e := fmt.Errorf("failed to get total input satoshis: %w", err)
		return validator.NewError(e, api.ErrStatusCumulativeFees)
	}
	totalOutputTx := tx.TotalOutputSatoshis()
	if totalOutputTx > totalInputTx {
		return validator.NewError(fmt.Errorf("total outputs %d is larger than total inputs %d for tx %s", totalOutputTx, totalInputTx, tx.TxID()), api.ErrStatusCumulativeFees)
	}

	paidFeeTx := totalInputTx - totalOutputTx

	for _, txFromSet := range txSet {
		expectedFeeTx, err := feeModel.ComputeFee(txFromSet)
		if err != nil {
			e := fmt.Errorf("failed to compute fee: %w", err)
			return validator.NewError(e, api.ErrStatusCumulativeFees)
		}
		expectedCumulativeFee += expectedFeeTx
		total, err := txFromSet.TotalInputSatoshis()
		if err != nil {
			e := fmt.Errorf("failed to get total input satoshis: %w", err)
			return validator.NewError(e, api.ErrStatusCumulativeFees)
		}
		totalInput := total
		totalOutput := txFromSet.TotalOutputSatoshis()

		if totalOutput > totalInput {
			return validator.NewError(fmt.Errorf("total outputs %d is larger than total inputs %d for tx %s", totalOutput, totalInput, txFromSet.TxID()), api.ErrStatusCumulativeFees)
		}

		cumulativePaidFeeAncestors += totalInput - totalOutput
	}

	actualCumulativeFee := paidFeeTx + cumulativePaidFeeAncestors

	if expectedCumulativeFee > paidFeeTx+cumulativePaidFeeAncestors {
		err = errors.Join(ErrTxFeeTooLow, fmt.Errorf("minimum expected cumulative fee: %d, actual cumulative fee: %d", expectedCumulativeFee, actualCumulativeFee))
		return validator.NewError(err, api.ErrStatusCumulativeFees)
	}

	return nil
}

func isFeePaidEnough(feeModel sdkTx.FeeModel, tx *sdkTx.Transaction) (bool, uint64, uint64, error) {
	expFeesPaid, err := feeModel.ComputeFee(tx)
	if err != nil {
		return false, 0, 0, err
	}

	total, err := tx.TotalInputSatoshis()
	if err != nil {
		return false, 0, 0, err
	}
	totalInputSatoshis := total
	totalOutputSatoshis := tx.TotalOutputSatoshis()

	if totalInputSatoshis < totalOutputSatoshis {
		return false, expFeesPaid, 0, nil
	}

	actualFeePaid := totalInputSatoshis - totalOutputSatoshis
	return actualFeePaid >= expFeesPaid, expFeesPaid, actualFeePaid, nil
}

func checkScripts(tx *sdkTx.Transaction) error {
	for i, in := range tx.Inputs {
		prevOutput := &sdkTx.TransactionOutput{
			Satoshis:      *in.SourceTxSatoshis(),
			LockingScript: in.SourceTxScript(),
		}

		if err := validator.CheckScript(tx, i, prevOutput); err != nil {
			return err
		}
	}

	return nil
}
