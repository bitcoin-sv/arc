package defaultvalidator

import (
	"context"
	"errors"
	"fmt"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/fees"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/api"
)

var (
	ErrTxFeeTooLow = fmt.Errorf("transaction fee is too low")
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
	ctx, span := tracing.StartTracing(ctx, "ValidateTransaction", tracingEnabled, tracingAttributes...)
	defer tracing.EndTracing(span)

	// 0) Check whether we have a complete transaction in extended format, with all input information
	//    we cannot check the satoshi input, OP_RETURN is allowed 0 satoshis
	if needsExtension(tx, feeValidation, scriptValidation) {
		err := extendTx(ctx, v.txFinder, tx, tracingEnabled, tracingAttributes...)
		if err != nil {
			return validator.NewError(err, api.ErrStatusTxFormat)
		}
	}

	// The rest of the validation steps
	err := validator.CommonValidateTransaction(v.policy, tx)
	if err != nil {
		return err
	}

	// 10) Reject if the sum of input values is less than sum of output values
	// 11) Reject if transaction fee would be too low (minRelayTxFee) to get into an empty block.
	switch feeValidation {
	case validator.StandardFeeValidation:
		if err := standardCheckFees(tx, api.FeesToFeeModel(v.policy.MinMiningTxFee)); err != nil {
			return err
		}
	case validator.CumulativeFeeValidation:
		if err := cumulativeCheckFees(ctx, v.txFinder, tx, api.FeesToFeeModel(v.policy.MinMiningTxFee), tracingEnabled, tracingAttributes...); err != nil {
			return err
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

func standardCheckFees(tx *sdkTx.Transaction, feeModel sdkTx.FeeModel) *validator.Error {
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

func cumulativeCheckFees(ctx context.Context, txFinder validator.TxFinderI, tx *sdkTx.Transaction, feeModel *fees.SatoshisPerKilobyte, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) *validator.Error {
	ctx, span := tracing.StartTracing(ctx, "cumulativeCheckFees", tracingEnabled, tracingAttributes...)
	defer tracing.EndTracing(span)
	txSet, err := getUnminedAncestors(ctx, txFinder, tx, tracingEnabled, tracingAttributes...)
	if err != nil {
		e := fmt.Errorf("getting all unmined ancestors for CFV failed. reason: %w. found: %d", err, len(txSet))
		return validator.NewError(e, api.ErrStatusCumulativeFees)
	}
	txSet[""] = tx // do not need to care about key in the set

	cumulativeSize := 0
	cumulativePaidFee := uint64(0)

	for _, tx := range txSet {
		cumulativeSize += tx.Size()
		cumulativePaidFee += tx.TotalInputSatoshis() - tx.TotalOutputSatoshis()
	}

	expectedFee, err := feeModel.ComputeFeeBasedOnSize(uint64(cumulativeSize))
	if err != nil {
		return validator.NewError(err, api.ErrStatusCumulativeFees)
	}

	if expectedFee > cumulativePaidFee {
		err = errors.Join(ErrTxFeeTooLow, fmt.Errorf("minimum expected cumulative fee: %d, actual fee: %d", expectedFee, cumulativePaidFee))
		return validator.NewError(err, api.ErrStatusCumulativeFees)
	}

	return nil
}

func isFeePaidEnough(feeModel sdkTx.FeeModel, tx *sdkTx.Transaction) (bool, uint64, uint64, error) {
	expFeesPaid, err := feeModel.ComputeFee(tx)
	if err != nil {
		return false, 0, 0, err
	}

	totalInputSatoshis := tx.TotalInputSatoshis()
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
