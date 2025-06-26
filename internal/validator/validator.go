package validator

import (
	"context"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
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

type DefaultValidator interface {
	ValidateTransaction(ctx context.Context, tx *sdkTx.Transaction, feeValidation FeeValidation, scriptValidation ScriptValidation, blockHeight int32) error
}

type BeefValidator interface {
	ValidateTransaction(ctx context.Context, beefTx *sdkTx.Beef, feeValidation FeeValidation, scriptValidation ScriptValidation) (failedTx *sdkTx.Transaction, err error)
}
