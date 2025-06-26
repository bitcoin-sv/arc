package validator

import (
	"errors"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"

	internalApi "github.com/bitcoin-sv/arc/internal/api"
	"github.com/bitcoin-sv/arc/pkg/api"
)

type FeeValidation byte

const (
	NoneFeeValidation FeeValidation = iota
	StandardFeeValidation
	CumulativeFeeValidation
)

const DustLimit = 1

type ScriptValidation byte

const (
	NoneScriptValidation ScriptValidation = iota
	StandardScriptValidation
)

var (
	ErrVerifyScriptFailed = errors.New("script verification failed")
	ErrExtendTx           = errors.New("failed to extend transaction")
)

type GenericValidator struct {
	scriptVerifier   internalApi.ScriptVerifier
	genesisForkBLock int32
}

func NewGenericValidator(scriptVerifier internalApi.ScriptVerifier, genesisForkBLock int32) *GenericValidator {
	return &GenericValidator{
		scriptVerifier:   scriptVerifier,
		genesisForkBLock: genesisForkBLock,
	}
}

func (v *GenericValidator) StandardScriptValidation(scriptValidation ScriptValidation, tx *sdkTx.Transaction, blockHeight int32) *Error { //nolint: revive //false error thrown
	switch scriptValidation {
	case StandardScriptValidation:
		utxo := make([]int32, len(tx.Inputs))
		for i := range tx.Inputs {
			utxo[i] = v.genesisForkBLock
		}

		b, err := tx.EF()
		if err != nil {
			return NewError(errors.Join(ErrExtendTx, err), api.ErrStatusMalformed)
		}

		err = v.scriptVerifier.VerifyScript(b, utxo, blockHeight, true)
		if err != nil {
			return NewError(errors.Join(ErrVerifyScriptFailed, err), api.ErrStatusUnlockingScripts)
		}
	case NoneScriptValidation:
		// No script validation
	}
	return nil
}
