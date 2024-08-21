package defaultvalidator

import (
	"context"
	"fmt"
	"github.com/bitcoin-sv/arc/internal/fees"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/ordishs/go-bitcoin"
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

func (v *DefaultValidator) ValidateTransaction(ctx context.Context, tx *transaction.Transaction, feeValidation validator.FeeValidation, scriptValidation validator.ScriptValidation) error { //nolint:funlen - mostly comments
	// 0) Check whether we have a complete transaction in extended format, with all input information
	//    we cannot check the satoshi input, OP_RETURN is allowed 0 satoshis
	if needsExtension(tx, feeValidation, scriptValidation) {
		err := extendTx(ctx, v.txFinder, tx)
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
		if err := cumulativeCheckFees(ctx, v.txFinder, tx, api.FeesToFeeModel(v.policy.MinMiningTxFee), v.policy.LimitCPFPGroupMembersCount); err != nil {
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

func needsExtension(tx *transaction.Transaction, fv validator.FeeValidation, sv validator.ScriptValidation) bool {
	// don't need if we don't validate fee AND scripts
	if fv == validator.NoneFeeValidation && sv == validator.NoneScriptValidation {
		return false
	}

	// don't need if is extended already
	return !isExtended(tx)
}

func isExtended(tx *transaction.Transaction) bool {
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

func standardCheckFees(tx *transaction.Transaction, feeModel transaction.FeeModel) *validator.Error {
	feesOK, expFeesPaid, actualFeePaid, err := isFeePaidEnough(feeModel, tx)
	if err != nil {
		return validator.NewError(err, api.ErrStatusFees)
	}

	if !feesOK {
		err = fmt.Errorf("transaction fee of %d sat is too low - minimum expected fee is %d sat", actualFeePaid, expFeesPaid)
		return validator.NewError(err, api.ErrStatusFees)
	}

	return nil
}

func cumulativeCheckFees(ctx context.Context, txFinder validator.TxFinderI, tx *transaction.Transaction, feeModel *fees.SatoshisPerKilobyte, unminedAncestorsLimit int) *validator.Error {
	txSet, err := getUnminedAncestors(ctx, txFinder, tx)
	if err != nil {
		return validator.NewError(err, api.ErrStatusCumulativeFees)
	}
	// the condition is the same as in the bitcoin node code: https://github.com/bitcoin-sv/bitcoin-sv/blob/master/src/txmempool.cpp#L228
	if len(txSet) >= unminedAncestorsLimit {
		return validator.NewError(fmt.Errorf("too many unconfirmed parents, %d [limit: %d]", len(txSet), unminedAncestorsLimit), api.ErrStatusCumulativeFees)
	}

	txSet[""] = tx // do not need to care about key in the set

	cumulativeSize := 0
	cumulativePaidFee := uint64(0)

	for _, tx := range txSet {
		cumulativeSize += tx.Size()
		//cumulativeSize.TotalBytes += size.TotalBytes
		//cumulativeSize.TotalDataBytes += size.TotalDataBytes
		//cumulativeSize.TotalStdBytes += size.TotalStdBytes

		cumulativePaidFee += tx.TotalInputSatoshis() - tx.TotalOutputSatoshis()
	}

	//expectedFee, err := validator.CalculateMiningFeesRequired(&cumulativeSize, feeQuote)
	expectedFee, err := feeModel.ComputeFeeBasedOnSize(uint64(cumulativeSize))
	if err != nil {
		return validator.NewError(err, api.ErrStatusCumulativeFees)
	}

	if expectedFee > cumulativePaidFee {
		err = fmt.Errorf("cumulative transaction fee of %d sat is too low - minimum expected fee is %d sat", cumulativePaidFee, expectedFee)
		return validator.NewError(err, api.ErrStatusCumulativeFees)
	}

	return nil
}

func isFeePaidEnough(feeModel transaction.FeeModel, tx *transaction.Transaction) (bool, uint64, uint64, error) {
	//expFeesPaid, err := validator.CalculateMiningFeesRequired(tx.SizeWithTypes(), fees)
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

func checkScripts(tx *transaction.Transaction) error {
	for i, in := range tx.Inputs {
		prevOutput := &transaction.TransactionOutput{
			Satoshis:      *in.SourceTxSatoshis(),
			LockingScript: in.SourceTxScript(),
		}

		if err := validator.CheckScript(tx, i, prevOutput); err != nil {
			return err
		}
	}

	return nil
}
