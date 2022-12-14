package defaultvalidator

import (
	"encoding/hex"
	"fmt"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/validator"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/bscript/interpreter"
)

// MaxBlockSize is set dynamically in a node, and should be gotten from the policy
const MaxBlockSize = 4 * 1024 * 1024 * 1024
const MaxSatoshis = 21_000_000_00_000_000
const coinbaseTxID = "0000000000000000000000000000000000000000000000000000000000000000"
const MaxTxSigopsCountPolicyAfterGenesis = ^uint32(0) // UINT32_MAX

type DefaultValidator struct {
	fees *api.FeesResponse
}

func New(fees *api.FeesResponse) validator.Validator {
	return &DefaultValidator{
		fees: fees,
	}
}

func (v *DefaultValidator) ValidateTransaction(tx *bt.Tx) error { //nolint:funlen - mostly comments
	//
	// Each node will verify every transaction against a long checklist of criteria:
	//
	txSize := tx.Size()

	// fmt.Println(hex.EncodeToString(tx.ExtendedBytes()))

	// 0) Check whether we have a complete transaction in extended format, with all input information
	//    we cannot check the satoshi input, OP_RETURN is allowed 0 satoshis
	for _, input := range tx.Inputs {
		if input.PreviousTxScript == nil || (input.PreviousTxSatoshis == 0 && !input.PreviousTxScript.IsData()) {
			return validator.NewError(fmt.Errorf("transaction is not in extended format"), api.ErrStatusTxFormat)
		}
	}

	// 1) Neither lists of inputs or outputs are empty
	if len(tx.Inputs) == 0 || len(tx.Outputs) == 0 {
		return validator.NewError(fmt.Errorf("transaction has no inputs or outputs"), api.ErrStatusInputs)
	}

	// 2) The transaction size in bytes is less than maxtxsizepolicy.
	if err := checkTxSize(txSize, v.fees); err != nil {
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
	if txSize < 100 {
		return validator.NewError(fmt.Errorf("transaction size in bytes is less than 100 bytes"), api.ErrStatusMalformed)
	}

	// 8) The number of signature operations (SIGOPS) contained in the transaction is less than the signature operation limit
	if err := sigOpsCheck(tx, v.fees); err != nil {
		return validator.NewError(err, api.ErrStatusMalformed)
	}

	// 9) The unlocking script (scriptSig) can only push numbers on the stack
	if err := pushDataCheck(tx); err != nil {
		return validator.NewError(err, api.ErrStatusMalformed)
	}

	// 10) Reject if the sum of input values is less than sum of output values
	// 11) Reject if transaction fee would be too low (minRelayTxFee) to get into an empty block.
	if err := checkFees(tx, api.FeesToBtFeeQuote(v.fees.Fees)); err != nil {
		return validator.NewError(err, api.ErrStatusFees)
	}

	// 12) The unlocking scripts for each input must validate against the corresponding output locking scripts
	if err := checkScripts(tx); err != nil {
		return validator.NewError(err, api.ErrStatusUnlockingScripts)
	}

	// everything checks out
	return nil
}

func checkTxSize(txSize int, policy *api.FeesResponse) error {
	maxTxSizePolicy := 0
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
		case !isData && (output.Satoshis > MaxSatoshis || output.Satoshis < api.DustThreshold):
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
		/* TODO lots of our valid test transactions have this sequence number, is this not allowed?
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
	feesOK, err := tx.IsFeePaidEnough(feeQuote)
	if err != nil {
		return err
	}

	if !feesOK {
		return fmt.Errorf("transaction fee is too low")
	}

	return nil
}

func sigOpsCheck(tx *bt.Tx, policy *api.FeesResponse) error {
	maxSigOps := 1
	if maxSigOps == 0 {
		maxSigOps = int(MaxTxSigopsCountPolicyAfterGenesis)
	}
	numSigOps := 0
	for _, input := range tx.Inputs {
		parser := interpreter.DefaultOpcodeParser{}
		parsedUnlockingScript, err := parser.Parse(input.PreviousTxScript)
		if err != nil {
			return err
		}

		for _, op := range parsedUnlockingScript {
			if op.Value() == bscript.OpCHECKSIG || op.Value() == bscript.OpCHECKSIGVERIFY {
				numSigOps++
				if numSigOps > maxSigOps {
					return fmt.Errorf("transaction unlocking scripts have too many sigops (%d)", numSigOps)
				}
			}
		}
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
