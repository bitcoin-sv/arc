package validator

import (
	"fmt"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/script/interpreter"
	"github.com/bitcoin-sv/go-sdk/transaction"
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

func CommonValidateTransaction(policy *bitcoin.Settings, tx *transaction.Transaction) *Error {
	//
	// Each node will verify every transaction against a long checklist of criteria:
	//
	txSize := tx.Size()

	// 1) Neither lists of inputs or outputs are empty
	if len(tx.Inputs) == 0 || len(tx.Outputs) == 0 {
		return NewError(fmt.Errorf("transaction has no inputs or outputs"), api.ErrStatusInputs)
	}

	// 2) The transaction size in bytes is less than maxtxsizepolicy.
	if err := checkTxSize(txSize, policy); err != nil {
		return NewError(err, api.ErrStatusTxFormat)
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
		return NewError(fmt.Errorf("transaction size in bytes is less than %d bytes", minTxSizeBytes), api.ErrStatusMalformed)
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
		return fmt.Errorf("transaction size in bytes is greater than max tx size policy %d", maxTxSizePolicy)
	}

	return nil
}

func checkOutputs(tx *transaction.Transaction) *Error {
	total := uint64(0)
	for index, output := range tx.Outputs {
		isData := output.LockingScript.IsData()
		switch {
		case !isData && (output.Satoshis > maxSatoshis || output.Satoshis < DustLimit):
			return NewError(fmt.Errorf("transaction output %d satoshis is invalid", index), api.ErrStatusOutputs)
		case isData && output.Satoshis != 0:
			return NewError(fmt.Errorf("transaction output %d has non 0 value op return", index), api.ErrStatusOutputs)
		}
		total += output.Satoshis
	}

	if total > maxSatoshis {
		return NewError(fmt.Errorf("transaction output total satoshis is too high"), api.ErrStatusOutputs)
	}

	return nil
}

func checkInputs(tx *transaction.Transaction) *Error {
	total := uint64(0)
	for index, input := range tx.Inputs {
		if input.PreviousTxIDStr() == coinbaseTxID {
			return NewError(fmt.Errorf("transaction input %d is a coinbase input", index), api.ErrStatusInputs)
		}

		inputSatoshis := uint64(0)
		if input.SourceTxSatoshis() != nil {
			inputSatoshis = *input.SourceTxSatoshis()
		}

		if inputSatoshis > maxSatoshis {
			return NewError(fmt.Errorf("transaction input %d satoshis is too high", index), api.ErrStatusInputs)
		}
		total += inputSatoshis
	}
	if total > maxSatoshis {
		return NewError(fmt.Errorf("transaction input total satoshis is too high"), api.ErrStatusInputs)
	}

	return nil
}

func sigOpsCheck(tx *transaction.Transaction, policy *bitcoin.Settings) error {
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
		return fmt.Errorf("transaction unlocking scripts have too many sigops (%d)", numSigOps)
	}

	return nil
}

func pushDataCheck(tx *transaction.Transaction) error {
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

func CheckScript(tx *transaction.Transaction, inputIdx int, prevTxOutput *transaction.TransactionOutput) error {
	err := interpreter.NewEngine().Execute(
		interpreter.WithTx(tx, inputIdx, prevTxOutput),
		interpreter.WithForkID(),
		interpreter.WithAfterGenesis(),
	)

	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}
