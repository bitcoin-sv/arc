package validator

import (
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/script/interpreter"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/ordishs/go-bitcoin"
)

// maxBlockSize is set dynamically in a node, and should be gotten from the policy
const (
	maxBlockSize                       = 4 * 1024 * 1024 * 1024
	maxSatoshis                        = 21_000_000_00_000_000
	coinbaseTxID                       = "0000000000000000000000000000000000000000000000000000000000000000"
	maxTxSigopsCountPolicyAfterGenesis = ^uint32(0) // UINT32_MAX
	minTxSizeBytes                     = 61
)

var (
	ErrNoInputsOrOutputs               = errors.New("transaction has no inputs or outputs")
	ErrTxOutputInvalid                 = errors.New("transaction output is invalid")
	ErrTxInputInvalid                  = errors.New("transaction input is invalid")
	ErrUnlockingScriptHasTooManySigOps = errors.New("transaction unlocking scripts have too many sigops")
	ErrEmptyUnlockingScript            = errors.New("transaction input unlocking script is empty")
	ErrUnlockingScriptNotPushOnly      = errors.New("transaction input unlocking script is not push only")
	ErrScriptExecutionFailed           = errors.New("script execution failed")

	ErrTxSizeLessThanMinSize = fmt.Errorf("transaction size in bytes is less than %d bytes", minTxSizeBytes)
	ErrTxSizeGreaterThanMax  = fmt.Errorf("transaction size in bytes is greater than %d bytes", maxBlockSize)
)

func CommonValidateTransaction(policy *bitcoin.Settings, tx *sdkTx.Transaction) *Error {
	//
	// Each node will verify every transaction against a long checklist of criteria:
	//
	txSize := tx.Size()

	// 1) Neither lists of inputs or outputs are empty
	if len(tx.Inputs) == 0 || len(tx.Outputs) == 0 {
		return NewError(ErrNoInputsOrOutputs, api.ErrStatusInputs)
	}

	// 2) The transaction size in bytes is less than maxtxsizepolicy.
	if err := checkTxSize(txSize, policy); err != nil {
		return NewError(err, api.ErrStatusTxSize)
	}

	// 3) check that each input value, as well as the sum, are in the allowed range of values (less than 21m coins)
	// 5) None of the inputs have hash=0, N=–1 (coinbase transactions should not be relayed)
	if err := checkInputs(tx); err != nil {
		return NewError(err, api.ErrStatusInputs)
	}

	// 4) Each output value, as well as the total, must be within the allowed range of values (less than 21m coins,
	//    more than the dust threshold if 1 unless it's OP_RETURN, which is allowed to be 0)
	if err := checkOutputs(tx); err != nil {
		return NewError(err, api.ErrStatusOutputs)
	}

	// 6) nLocktime is equal to INT_MAX, or nLocktime and nSequence values are satisfied according to MedianTimePast
	//    => checked by the node, we do not want to have to know the current block height

	// 7) The transaction size in bytes is greater than or equal to 100
	if txSize < minTxSizeBytes {
		return NewError(ErrTxSizeLessThanMinSize, api.ErrStatusMalformed)
	}

	// 8) The number of signature operations (SIGOPS) contained in the transaction is less than the signature operation limit
	if err := sigOpsCheck(tx, policy); err != nil {
		return NewError(err, api.ErrStatusMalformed)
	}

	// 9) The unlocking script (scriptSig) can only push numbers on the stack
	if err := pushDataCheck(tx); err != nil {
		return NewError(err, api.ErrStatusMalformed)
	}

	// everything checks out
	return nil
}

func checkTxSize(txSize int, policy *bitcoin.Settings) error {
	maxTxSizePolicy := policy.MaxTxSizePolicy
	if maxTxSizePolicy == 0 {
		// no policy found for tx size, use max block size
		maxTxSizePolicy = maxBlockSize
	}
	if txSize > maxTxSizePolicy {
		return ErrTxSizeGreaterThanMax
	}

	return nil
}

func checkOutputs(tx *sdkTx.Transaction) *Error {
	total := uint64(0)
	for index, output := range tx.Outputs {
		isData := output.LockingScript.IsData()
		switch {
		case !isData && (output.Satoshis > maxSatoshis || output.Satoshis < DustLimit):
			return NewError(errors.Join(ErrTxOutputInvalid, fmt.Errorf("output %d satoshis is invalid", index)), api.ErrStatusOutputs)
		case isData && output.Satoshis != 0:
			return NewError(errors.Join(ErrTxOutputInvalid, fmt.Errorf("output %d has non 0 value op return", index)), api.ErrStatusOutputs)
		}
		total += output.Satoshis
	}

	if total > maxSatoshis {
		return NewError(errors.Join(ErrTxOutputInvalid, fmt.Errorf("output total satoshis is too high")), api.ErrStatusOutputs)
	}

	return nil
}

func checkInputs(tx *sdkTx.Transaction) *Error {
	total := uint64(0)
	for index, input := range tx.Inputs {
		if input.PreviousTxIDStr() == coinbaseTxID {
			return NewError(errors.Join(ErrTxInputInvalid, fmt.Errorf("input %d is a coinbase input", index)), api.ErrStatusInputs)
		}

		inputSatoshis := uint64(0)
		if input.SourceTxSatoshis() != nil {
			inputSatoshis = *input.SourceTxSatoshis()
		}

		if inputSatoshis > maxSatoshis {
			return NewError(errors.Join(ErrTxInputInvalid, fmt.Errorf("input %d satoshis is too high", index)), api.ErrStatusInputs)
		}
		total += inputSatoshis
	}
	if total > maxSatoshis {
		return NewError(errors.Join(ErrTxInputInvalid, fmt.Errorf("input total satoshis is too high")), api.ErrStatusInputs)
	}

	return nil
}

func sigOpsCheck(tx *sdkTx.Transaction, policy *bitcoin.Settings) error {
	maxSigOps := policy.MaxTxSigopsCountsPolicy

	if maxSigOps == 0 {
		maxSigOps = int64(maxTxSigopsCountPolicyAfterGenesis)
	}

	parser := interpreter.DefaultOpcodeParser{}
	numSigOps := int64(0)

	for _, input := range tx.Inputs {
		parsedUnlockingScript, err := parser.Parse(input.UnlockingScript)
		if err != nil {
			return err
		}

		for _, op := range parsedUnlockingScript {
			if op.Value() == script.OpCHECKSIG || op.Value() == script.OpCHECKSIGVERIFY {
				numSigOps++
			}
		}
	}

	for _, output := range tx.Outputs {
		parsedLockingScript, err := parser.Parse(output.LockingScript)
		if err != nil {
			return err
		}

		for _, op := range parsedLockingScript {
			if op.Value() == script.OpCHECKSIG || op.Value() == script.OpCHECKSIGVERIFY {
				numSigOps++
			}
		}
	}

	if numSigOps > maxSigOps {
		return errors.Join(ErrUnlockingScriptHasTooManySigOps, fmt.Errorf("sigops: %d", numSigOps))
	}

	return nil
}

func pushDataCheck(tx *sdkTx.Transaction) error {
	for index, input := range tx.Inputs {
		if input.UnlockingScript == nil {
			return errors.Join(ErrEmptyUnlockingScript, fmt.Errorf("input: %d", index))
		}
		parser := interpreter.DefaultOpcodeParser{}
		parsedUnlockingScript, err := parser.Parse(input.UnlockingScript)
		if err != nil {
			return err
		}
		if !parsedUnlockingScript.IsPushOnly() {
			return errors.Join(ErrUnlockingScriptNotPushOnly, fmt.Errorf("input: %d", index))
		}
	}

	return nil
}

func CheckScript(tx *sdkTx.Transaction, inputIdx int, prevTxOutput *sdkTx.TransactionOutput) error {
	err := interpreter.NewEngine().Execute(
		interpreter.WithTx(tx, inputIdx, prevTxOutput),
		interpreter.WithForkID(),
		interpreter.WithAfterGenesis(),
	)

	if err != nil {
		return errors.Join(ErrScriptExecutionFailed, err)
	}

	return nil
}
