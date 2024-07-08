package defaultvalidator

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/validator"
	validation "github.com/bitcoin-sv/arc/internal/validator/internal"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript/interpreter"
	"github.com/ordishs/go-bitcoin"
)

type DefaultValidator struct {
	policy *bitcoin.Settings
}

func New(policy *bitcoin.Settings) *DefaultValidator {
	return &DefaultValidator{
		policy: policy,
	}
}

func (v *DefaultValidator) ValidateTransaction(ctx context.Context, tx *bt.Tx, feeValidation validator.FeeValidation, scriptValidation validator.ScriptValidation) error { //nolint:funlen - mostly comments
	// 0) Check whether we have a complete transaction in extended format, with all input information
	//    we cannot check the satoshi input, OP_RETURN is allowed 0 satoshis
	if !v.IsExtended(tx) {
		return validation.NewError(fmt.Errorf("transaction is not in extended format"), api.ErrStatusTxFormat)
	}

	// The rest of the validation steps
	err := validation.CommonValidateTransaction(v.policy, tx)
	if err != nil {
		return err
	}

	// 10) Reject if the sum of input values is less than sum of output values
	// 11) Reject if transaction fee would be too low (minRelayTxFee) to get into an empty block.
	switch feeValidation {
	case validator.StandardFeeValidation:
		if err := standardCheckFees(tx, api.FeesToBtFeeQuote(v.policy.MinMiningTxFee)); err != nil {
			return err
		}
	case validator.DeepFeeValidation:
		if err := deepCheckFees(ctx, tx, api.FeesToBtFeeQuote(v.policy.MinMiningTxFee)); err != nil {
			return err
		}
	case validator.NoneFeeValidation:
		fallthrough
	default:
	}

	// 12) The unlocking scripts for each input must validate against the corresponding output locking scripts
	if scriptValidation == validator.StandardScriptValidation {
		if err := checkScripts(tx); err != nil {
			return validation.NewError(err, api.ErrStatusUnlockingScripts)
		}
	}

	// everything checks out
	return nil
}

// TODO: morph to EF
func (v *DefaultValidator) IsExtended(tx *bt.Tx) bool {
	if tx == nil || tx.Inputs == nil {
		return false
	}

	for _, input := range tx.Inputs {
		if input.PreviousTxScript == nil || (input.PreviousTxSatoshis == 0 && !input.PreviousTxScript.IsData()) {
			return false
		}
	}

	return true
}

func standardCheckFees(tx *bt.Tx, feeQuote *bt.FeeQuote) error {
	feesOK, expFeesPaid, actualFeePaid, err := isFeePaidEnough(feeQuote, tx)
	if err != nil {
		return validation.NewError(err, api.ErrStatusFees)
	}

	if !feesOK {
		err = fmt.Errorf("transaction fee of %d sat is too low - minimum expected fee is %d sat", actualFeePaid, expFeesPaid)
		return validation.NewError(err, api.ErrStatusFees)
	}

	return nil
}

func isFeePaidEnough(fees *bt.FeeQuote, tx *bt.Tx) (bool, uint64, uint64, error) {
	expFeesPaid, err := validation.CalculateMiningFeesRequired(tx.SizeWithTypes(), fees)
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

func deepCheckFees(ctx context.Context, tx *bt.Tx, feeQuote *bt.FeeQuote) error {
	// TODO: change/refactor in scope of the ARCO-147
	txFinder := validator.NewTxFinder()

	txSet, err := validator.GetUnminedAncestors(ctx, txFinder, tx)
	if err != nil {
		return validation.NewError(err, api.ErrStatusFees) // TODO: add new status
	}
	txSet[""] = tx // do not need to care about key in the set

	cumulativeExpFee := uint64(0)
	cumulativePaidFee := uint64(0)

	for _, tx := range txSet {
		expFees, err := validation.CalculateMiningFeesRequired(tx.SizeWithTypes(), feeQuote)
		if err != nil {
			return validation.NewError(err, api.ErrStatusFees) // TODO: add new status
		}

		cumulativeExpFee += expFees
		cumulativePaidFee += (tx.TotalInputSatoshis() - tx.TotalOutputSatoshis())
	}

	if cumulativeExpFee > cumulativePaidFee {
		err = fmt.Errorf("cumulative transaction fee of %d sat is too low - minimum expected fee is %d sat", cumulativePaidFee, cumulativeExpFee)
		return validation.NewError(err, api.ErrStatusFees) // TODO: add new status
	}

	return nil
}

// TODO move to common
func checkScripts(tx *bt.Tx) error {
	for i, in := range tx.Inputs {
		prevOutput := &bt.Output{
			Satoshis:      in.PreviousTxSatoshis,
			LockingScript: in.PreviousTxScript,
		}

		if err := interpreter.NewEngine().Execute(
			interpreter.WithTx(tx, i, prevOutput),
			interpreter.WithForkID(),
			interpreter.WithAfterGenesis(),
		); err != nil {
			return fmt.Errorf("script execution failed: %w", err)
		}
	}

	return nil
}
