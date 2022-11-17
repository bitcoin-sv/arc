package defaultvalidator

import (
	"fmt"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/validator"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript/interpreter"
)

// MaxBlockSize is set dynamically in a node, and should be gotten from the policy
const MaxBlockSize = 4 * 1024 * 1024 * 1024

type DefaultValidator struct {
	policy *mapi.Policy
}

func New(policy *mapi.Policy) validator.Validator {
	return &DefaultValidator{
		policy: policy,
	}
}

func (v *DefaultValidator) ValidateTransaction(tx *bt.Tx) error {
	//
	// Each node will verify every transaction against a long checklist of criteria:
	//
	txSize := tx.Size()

	// 1) The transaction's syntax and data structure must be correct
	//    => this has been checked by the calling function, when loading go-bt TX

	// 2) Neither lists of inputs or outputs are empty
	if len(tx.Inputs) == 0 || len(tx.Outputs) == 0 {
		return validator.NewError(fmt.Errorf("transaction has no inputs or outputs"), mapi.ErrStatusInputs)
	}

	// 3) The transaction size in bytes is less than maxtxsizepolicy.
	maxTxSizePolicy := mapi.GetPolicyInt(v.policy, "maxtxsizepolicy")
	if maxTxSizePolicy == 0 {
		// no policy found for tx size, use max block size
		maxTxSizePolicy = MaxBlockSize
	}
	if txSize > maxTxSizePolicy {
		return validator.NewError(fmt.Errorf("transaction size in bytes is greater than MAX_BLOCK_SIZE"), mapi.ErrStatusMalformed)
	}

	// 4) check that each input value, as well as the sum, are in the allowed range of values (less than 21m coins, more than 0)

	// 5) Each output value, as well as the total, must be within the allowed range of values (less than 21m coins,
	//    more than the dust threshold if 1 unless it's OP_RETURN, which is allowed to be 0)

	// 6) None of the inputs have hash=0, N=–1 (coinbase transactions should not be relayed)

	// 7) nLocktime is equal to INT_MAX, or nLocktime and nSequence values are satisfied according to MedianTimePast

	// 8) The transaction size in bytes is greater than or equal to 100
	if txSize < 100 {
		return validator.NewError(fmt.Errorf("transaction size in bytes is less than 100 bytes"), mapi.ErrStatusMalformed)
	}

	// 9) The number of signature operations (SIGOPS) contained in the transaction is less than the signature operation limit

	// 10) The unlocking script (scriptSig) can only push numbers on the stack

	// 11) Reject if the sum of input values is less than sum of output values
	// 12) Reject if transaction fee would be too low (minRelayTxFee) to get into an empty block.
	if err := checkFees(tx, mapi.FeesToBtFeeQuote(v.policy.Fees)); err != nil {
		return validator.NewError(err, mapi.ErrStatusFees)
	}

	// 13) The unlocking scripts for each input must validate against the corresponding output locking scripts
	if err := checkScripts(tx); err != nil {
		return validator.NewError(err, mapi.ErrStatusUnlockingScripts)
	}

	// everything checks out
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
