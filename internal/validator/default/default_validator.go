package defaultvalidator

import (
	"encoding/hex"
	"fmt"
	"math"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/bscript/interpreter"
	"github.com/ordishs/go-bitcoin"
)

// MaxBlockSize is set dynamically in a node, and should be gotten from the policy
const (
	MaxBlockSize                       = 4 * 1024 * 1024 * 1024
	MaxSatoshis                        = 21_000_000_00_000_000
	coinbaseTxID                       = "0000000000000000000000000000000000000000000000000000000000000000"
	MaxTxSigopsCountPolicyAfterGenesis = ^uint32(0) // UINT32_MAX
	minTxSizeBytes                     = 61
)

type DefaultValidator struct {
	policy *bitcoin.Settings
}

func New(policy *bitcoin.Settings) validator.Validator {
	return &DefaultValidator{
		policy: policy,
	}
}

func (v *DefaultValidator) ValidateEFTransaction(tx *bt.Tx, skipFeeValidation bool, skipScriptValidation bool) error { //nolint:funlen - mostly comments
	// 0) Check whether we have a complete transaction in extended format, with all input information
	//    we cannot check the satoshi input, OP_RETURN is allowed 0 satoshis
	if !v.IsExtended(tx) {
		return validator.NewError(fmt.Errorf("transaction is not in extended format"), api.ErrStatusTxFormat)
	}

	// The rest of the validation steps
	err := v.validateTransaction(tx, skipFeeValidation, skipScriptValidation)
	if err != nil {
		return err
	}

	// everything checks out
	return nil
}

func (v *DefaultValidator) ValidateBeef(beefTx *beef.BEEF, skipFeeValidation, skipScriptValidation bool) error {
	for _, btx := range beefTx.Transactions {
		if btx.Unmined() {
			tx := btx.Transaction

			// needs to be calculated here, because txs in beef are not in EF format
			if !skipFeeValidation {
				if err := v.validateBeefFees(tx, beefTx); err != nil {
					return err
				}
			}

			// needs to be calculated here, because txs in beef are not in EF format
			if !skipScriptValidation {
				if err := beef.ValidateScripts(tx, beefTx.Transactions); err != nil {
					return validator.NewError(err, api.ErrStatusUnlockingScripts)
				}
			}

			// purposefully skip the fee and scripts validation, because it's done above
			if err := v.validateTransaction(tx, true, true); err != nil {
				return err
			}
		}
	}

	if err := beef.EnsureAncestorsArePresentInBump(beefTx.GetLatestTx(), beefTx); err != nil {
		return validator.NewError(err, api.ErrBeefMinedAncestorsNotFound)
	}

	return nil
}

func (v *DefaultValidator) validateTransaction(tx *bt.Tx, skipFeeValidation, skipScriptValidation bool) error {
	//
	// Each node will verify every transaction against a long checklist of criteria:
	//
	txSize := tx.Size()

	// 1) Neither lists of inputs or outputs are empty
	if len(tx.Inputs) == 0 || len(tx.Outputs) == 0 {
		return validator.NewError(fmt.Errorf("transaction has no inputs or outputs"), api.ErrStatusInputs)
	}

	// 2) The transaction size in bytes is less than maxtxsizepolicy.
	if err := checkTxSize(txSize, v.policy); err != nil {
		return validator.NewError(err, api.ErrStatusTxFormat)
	}

	// 3) check that each input value, as well as the sum, are in the allowed range of values (less than 21m coins)
	// 5) None of the inputs have hash=0, N=–1 (coinbase transactions should not be relayed)
	if err := checkInputs(tx); err != nil {
		return validator.NewError(err, api.ErrStatusInputs)
	}

	// 4) Each output value, as well as the total, must be within the allowed range of values (less than 21m coins,
	//    more than the dust threshold if 1 unless it's OP_RETURN, which is allowed to be 0)
	if err := checkOutputs(tx); err != nil {
		return validator.NewError(err, api.ErrStatusOutputs)
	}

	// 6) nLocktime is equal to INT_MAX, or nLocktime and nSequence values are satisfied according to MedianTimePast
	//    => checked by the node, we do not want to have to know the current block height

	// 7) The transaction size in bytes is greater than or equal to 100
	if txSize < minTxSizeBytes {
		return validator.NewError(fmt.Errorf("transaction size in bytes is less than %d bytes", minTxSizeBytes), api.ErrStatusMalformed)
	}

	// 8) The number of signature operations (SIGOPS) contained in the transaction is less than the signature operation limit
	if err := sigOpsCheck(tx, v.policy); err != nil {
		return validator.NewError(err, api.ErrStatusMalformed)
	}

	// 9) The unlocking script (scriptSig) can only push numbers on the stack
	if err := pushDataCheck(tx); err != nil {
		return validator.NewError(err, api.ErrStatusMalformed)
	}

	// 10) Reject if the sum of input values is less than sum of output values
	// 11) Reject if transaction fee would be too low (minRelayTxFee) to get into an empty block.
	if !skipFeeValidation {
		if err := checkFees(tx, api.FeesToBtFeeQuote(v.policy.MinMiningTxFee)); err != nil {
			return validator.NewError(err, api.ErrStatusFees)
		}
	}

	// 12) The unlocking scripts for each input must validate against the corresponding output locking scripts
	if !skipScriptValidation {
		if err := checkScripts(tx); err != nil {
			return validator.NewError(err, api.ErrStatusUnlockingScripts)
		}
	}

	// everything checks out
	return nil
}

func (v *DefaultValidator) validateBeefFees(tx *bt.Tx, beefTx *beef.BEEF) error {
	expectedFees, err := calculateMiningFeesRequired(tx.SizeWithTypes(), api.FeesToBtFeeQuote(v.policy.MinMiningTxFee))
	if err != nil {
		return validator.NewError(err, api.ErrStatusFees)
	}

	inputSatoshis, outputSatoshis, err := beef.CalculateInputsOutputsSatoshis(tx, beefTx.Transactions)
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

func (v *DefaultValidator) IsBeef(txHex []byte) bool {
	return beef.CheckBeefFormat(txHex)
}

func checkTxSize(txSize int, policy *bitcoin.Settings) error {
	maxTxSizePolicy := policy.MaxTxSizePolicy
	if maxTxSizePolicy == 0 {
		// no policy found for tx size, use max block size
		maxTxSizePolicy = MaxBlockSize
	}
	if txSize > maxTxSizePolicy {
		return fmt.Errorf("transaction size in bytes is greater than max tx size policy %d", maxTxSizePolicy)
	}

	return nil
}

func checkOutputs(tx *bt.Tx) error {
	total := uint64(0)
	for index, output := range tx.Outputs {
		isData := output.LockingScript.IsData()
		switch {
		case !isData && (output.Satoshis > MaxSatoshis || output.Satoshis < bt.DustLimit):
			return validator.NewError(fmt.Errorf("transaction output %d satoshis is invalid", index), api.ErrStatusOutputs)
		case isData && output.Satoshis != 0:
			return validator.NewError(fmt.Errorf("transaction output %d has non 0 value op return", index), api.ErrStatusOutputs)
		}
		total += output.Satoshis
	}

	if total > MaxSatoshis {
		return validator.NewError(fmt.Errorf("transaction output total satoshis is too high"), api.ErrStatusOutputs)
	}

	return nil
}

func checkInputs(tx *bt.Tx) error {
	total := uint64(0)
	for index, input := range tx.Inputs {
		if hex.EncodeToString(input.PreviousTxID()) == coinbaseTxID {
			return validator.NewError(fmt.Errorf("transaction input %d is a coinbase input", index), api.ErrStatusInputs)
		}
		/* lots of our valid test transactions have this sequence number, is this not allowed?
		if input.SequenceNumber == 0xffffffff {
			fmt.Printf("input %d has sequence number 0xffffffff, txid = %s", index, tx.TxID())
			return validator.NewError(fmt.Errorf("transaction input %d sequence number is invalid", index), arc.ErrStatusInputs)
		}
		*/
		if input.PreviousTxSatoshis > MaxSatoshis {
			return validator.NewError(fmt.Errorf("transaction input %d satoshis is too high", index), api.ErrStatusInputs)
		}
		total += input.PreviousTxSatoshis
	}
	if total > MaxSatoshis {
		return validator.NewError(fmt.Errorf("transaction input total satoshis is too high"), api.ErrStatusInputs)
	}

	return nil
}

func checkFees(tx *bt.Tx, feeQuote *bt.FeeQuote) error {
	feesOK, expFeesPaid, actualFeePaid, err := isFeePaidEnough(feeQuote, tx)
	if err != nil {
		return err
	}

	if !feesOK {
		return fmt.Errorf("transaction fee of %d sat is too low - minimum expected fee is %d sat", actualFeePaid, expFeesPaid)
	}

	return nil
}

func isFeePaidEnough(fees *bt.FeeQuote, tx *bt.Tx) (bool, uint64, uint64, error) {
	expFeesPaid, err := calculateMiningFeesRequired(tx.SizeWithTypes(), fees)
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

func calculateMiningFeesRequired(size *bt.TxSize, fees *bt.FeeQuote) (uint64, error) {
	var feesRequired float64

	feeStandard, err := fees.Fee(bt.FeeTypeStandard)
	if err != nil {
		return 0, err
	}

	feesRequired += float64(size.TotalStdBytes) * float64(feeStandard.MiningFee.Satoshis) / float64(feeStandard.MiningFee.Bytes)

	feeData, err := fees.Fee(bt.FeeTypeData)
	if err != nil {
		return 0, err
	}

	feesRequired += float64(size.TotalDataBytes) * float64(feeData.MiningFee.Satoshis) / float64(feeData.MiningFee.Bytes)

	// the minimum fees required is 1 satoshi
	feesRequiredRounded := uint64(math.Round(feesRequired))
	if feesRequiredRounded < 1 {
		feesRequiredRounded = 1
	}

	return feesRequiredRounded, nil
}

func sigOpsCheck(tx *bt.Tx, policy *bitcoin.Settings) error {
	maxSigOps := policy.MaxTxSigopsCountsPolicy

	if maxSigOps == 0 {
		maxSigOps = int64(MaxTxSigopsCountPolicyAfterGenesis)
	}

	numSigOps := int64(0)
	for _, input := range tx.Inputs {
		parser := interpreter.DefaultOpcodeParser{}
		parsedLockingScript, err := parser.Parse(input.UnlockingScript)
		if err != nil {
			return err
		}

		for _, op := range parsedLockingScript {
			if op.Value() == bscript.OpCHECKSIG || op.Value() == bscript.OpCHECKSIGVERIFY {
				numSigOps++
			}
		}
	}

	for _, output := range tx.Outputs {
		parser := interpreter.DefaultOpcodeParser{}
		parsedLockingScript, err := parser.Parse(output.LockingScript)
		if err != nil {
			return err
		}

		for _, op := range parsedLockingScript {
			if op.Value() == bscript.OpCHECKSIG || op.Value() == bscript.OpCHECKSIGVERIFY {
				numSigOps++
			}
		}
	}

	if numSigOps > maxSigOps {
		return fmt.Errorf("transaction unlocking scripts have too many sigops (%d)", numSigOps)
	}

	return nil
}

func pushDataCheck(tx *bt.Tx) error {
	for index, input := range tx.Inputs {
		if input.UnlockingScript == nil {
			return fmt.Errorf("transaction input %d unlocking script is empty", index)
		}
		parser := interpreter.DefaultOpcodeParser{}
		parsedUnlockingScript, err := parser.Parse(input.UnlockingScript)
		if err != nil {
			return err
		}
		if !parsedUnlockingScript.IsPushOnly() {
			return fmt.Errorf("transaction input %d unlocking script is not push only", index)
		}
	}

	return nil
}

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
			// interpreter.WithDebugger(&LogDebugger{}),
		); err != nil {
			return fmt.Errorf("script execution failed: %w", err)
		}
	}

	return nil
}
