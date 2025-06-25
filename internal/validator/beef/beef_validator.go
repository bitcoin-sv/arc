package beef

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
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
	ErrBEEFVerificationFailed = errors.New("BEEF verification failed")
)

type ChainTracker interface {
	IsValidRootForHeight(root *chainhash.Hash, height uint32) (bool, error)
}

type Validator struct {
	policy       *bitcoin.Settings
	chainTracker ChainTracker
}

func New(policy *bitcoin.Settings, chaintracker ChainTracker) *Validator {
	return &Validator{
		policy:       policy,
		chainTracker: chaintracker,
	}
}

func (v *Validator) ValidateTransaction(ctx context.Context, beefTx *sdkTx.Beef, txID string, feeValidation validator.FeeValidation, scriptValidation validator.ScriptValidation, tracingEnabled bool, tracingAttributes ...attribute.KeyValue) error {
	var vErr *validator.Error
	var spanErr error
	_, span := tracing.StartTracing(ctx, "BEEFValidator_ValidateTransaction", tracingEnabled, tracingAttributes...)
	defer func() {
		if vErr != nil {
			spanErr = vErr.Err
		}
		tracing.EndTracing(span, spanErr)
	}()

	for _, btx := range beefTx.Transactions {
		// verify only unmined transactions

		// check if is mined
		if btx.Transaction.TxID().String() != txID {
			continue
		}

		tx := btx.Transaction

		vErr = validator.CommonValidateTransaction(v.policy, tx)
		if vErr != nil {
			return vErr
		}

		if feeValidation == validator.StandardFeeValidation {
			vErr = standardCheckFees(tx, internalApi.FeesToFeeModel(v.policy.MinMiningTxFee))
			if vErr != nil {
				return vErr
			}
		}

		if scriptValidation == validator.StandardScriptValidation {
			vErr = validateScripts(beefTx, btx)
			if vErr != nil {
				return vErr
			}
		}
	}

	if feeValidation == validator.CumulativeFeeValidation {
		vErr = cumulativeCheckFees(beefTx, internalApi.FeesToFeeModel(v.policy.MinMiningTxFee))
		if vErr != nil {
			return vErr
		}
	}

	// verify with chain tracker
	ok, err := beefTx.Verify(v.chainTracker, false)
	if err != nil {
		vErr = validator.NewError(err, api.ErrStatusBeefValidationMerkleRoots)
		return vErr
	}

	if !ok {
		return validator.NewError(ErrBEEFVerificationFailed, api.ErrStatusBeefValidationFailedBeefInvalid)
	}

	return nil
}

func standardCheckFees(tx *sdkTx.Transaction, feeModel sdkTx.FeeModel) *validator.Error {
	expectedFees, err := feeModel.ComputeFee(tx)
	if err != nil {
		return validator.NewError(err, api.ErrStatusFees)
	}
	outputSatoshis := tx.TotalOutputSatoshis()
	inputSatoshis, err := tx.TotalInputSatoshis()
	if err != nil {
		return validator.NewError(err, api.ErrStatusFees)
	}

	actualFeePaid := inputSatoshis - outputSatoshis

	if inputSatoshis < outputSatoshis {
		// force an error without wrong negative values
		actualFeePaid = 0
	}

	if actualFeePaid < expectedFees {
		err := fmt.Errorf("transaction fee of %d sat is too low - minimum expected fee is %d sat", actualFeePaid, expectedFees)
		return validator.NewError(err, api.ErrStatusFees)
	}

	return nil
}

func cumulativeCheckFees(beefTx *sdkTx.Beef, feeModel *feemodel.SatoshisPerKilobyte) *validator.Error {
	cumulativePaidFee := uint64(0)
	expectedFees := uint64(0)

	for _, bTx := range beefTx.Transactions {
		if bTx.DataFormat != sdkTx.RawTx {
			continue
		}

		tx := bTx.Transaction

		totalOutputSatoshis := tx.TotalOutputSatoshis()
		totalInputSatoshis, err := tx.TotalInputSatoshis()
		if err != nil {
			return validator.NewError(err, api.ErrStatusCumulativeFees)
		}

		cumulativePaidFee += totalInputSatoshis - totalOutputSatoshis

		expectedFee, err := feeModel.ComputeFee(tx)
		if err != nil {
			return validator.NewError(err, api.ErrStatusCumulativeFees)
		}
		expectedFees += expectedFee
	}

	if expectedFees > cumulativePaidFee {
		err := fmt.Errorf("cumulative transaction fee of %d sat is too low - minimum expected fee is %d sat", cumulativePaidFee, expectedFees)
		return validator.NewError(err, api.ErrStatusCumulativeFees)
	}

	return nil
}

func validateScripts(beef *sdkTx.Beef, beefTx *sdkTx.BeefTx) *validator.Error {
	for i, input := range beefTx.Transaction.Inputs {
		inputTxID := input.SourceTXID.String()
		inputTx, ok := beef.Transactions[inputTxID]

		if !ok {
			return validator.NewError(errors.New("invalid script"), api.ErrStatusUnlockingScripts)
		}
		err := checkScripts(beefTx.Transaction, inputTx.Transaction, i)
		if err != nil {
			return validator.NewError(errors.New("invalid script"), api.ErrStatusUnlockingScripts)
		}
	}

	return nil
}

func checkScripts(tx *sdkTx.Transaction, prevTx *sdkTx.Transaction, inputIdx int) error {
	input := tx.InputIdx(inputIdx)
	prevOutput := prevTx.OutputIdx(int(input.SourceTxOutIndex))

	return validator.CheckScript(tx, inputIdx, prevOutput)
}
