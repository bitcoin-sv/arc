package beef

import (
	"context"
	"errors"
	"fmt"
	"runtime"

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
	ErrBEEFVerificationFailed   = errors.New("BEEF verification failed")
	ErrBEEFVerificationTimedOut = errors.New("BEEF verification timed out")

	ErrRequestFailed            = errors.New("request failed")
	ErrRequestTimedOut          = errors.New("request timed out")
	ErrNoChainTrackersAvailable = errors.New("no chain trackers available")
)

type ChainTracker interface {
	IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error)
	CurrentHeight(ctx context.Context) (uint32, error)
}

type Validator struct {
	policy            *bitcoin.Settings
	chainTracker      ChainTracker
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

type Option func(d *Validator)

func WithTracer(attr ...attribute.KeyValue) func(s *Validator) {
	return func(a *Validator) {
		a.tracingEnabled = true
		if len(attr) > 0 {
			a.tracingAttributes = append(a.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			a.tracingAttributes = append(a.tracingAttributes, attribute.String("file", file))
		}
	}
}

func New(policy *bitcoin.Settings, chainTracker ChainTracker, opts ...Option) *Validator {
	v := &Validator{
		policy:       policy,
		chainTracker: chainTracker,
	}
	// apply options
	for _, opt := range opts {
		opt(v)
	}

	return v
}

func (v *Validator) ValidateTransaction(ctx context.Context, beefTx *sdkTx.Beef, feeValidation validator.FeeValidation, scriptValidation validator.ScriptValidation) (failedTx *sdkTx.Transaction, err error) {
	var vErr *validator.Error
	var spanErr error
	_, span := tracing.StartTracing(ctx, "BEEFValidator_ValidateTransaction", v.tracingEnabled, v.tracingAttributes...)
	defer func() {
		if vErr != nil {
			spanErr = vErr.Err
		}
		tracing.EndTracing(span, spanErr)
	}()

	for _, btx := range beefTx.Transactions {
		// verify only unmined transactions

		if btx.DataFormat != sdkTx.RawTx {
			continue
		}

		tx := btx.Transaction

		vErr = validator.CommonValidateTransaction(v.policy, tx)
		if vErr != nil {
			return tx, vErr
		}

		if feeValidation == validator.StandardFeeValidation {
			vErr = standardCheckFees(tx, internalApi.FeesToFeeModel(v.policy.MinMiningTxFee))
			if vErr != nil {
				return tx, vErr
			}
		}

		if scriptValidation == validator.StandardScriptValidation {
			vErr = validateScripts(beefTx, btx)
			if vErr != nil {
				return tx, vErr
			}
		}
	}

	if feeValidation == validator.CumulativeFeeValidation {
		vErr = cumulativeCheckFees(beefTx, internalApi.FeesToFeeModel(v.policy.MinMiningTxFee))
		if vErr != nil {
			return nil, vErr
		}
	}

	var verificationSuccessful bool
	// verify with chain tracker
	verificationSuccessful, err = beefTx.Verify(ctx, v.chainTracker, false)
	if err != nil {
		if errors.Is(err, ErrRequestTimedOut) {
			return nil, validator.NewError(errors.Join(ErrBEEFVerificationTimedOut, err), api.ErrStatusBeefValidationMerkleRoots)
		}

		if errors.Is(err, ErrRequestFailed) {
			return nil, validator.NewError(errors.Join(ErrBEEFVerificationFailed, err), api.ErrStatusBeefValidationMerkleRoots)
		}

		return nil, validator.NewError(err, api.ErrStatusBeefValidationMerkleRoots)
	}

	if !verificationSuccessful {
		return nil, validator.NewError(ErrBEEFVerificationFailed, api.ErrStatusBeefValidationFailedBeefInvalid)
	}

	return nil, nil
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
