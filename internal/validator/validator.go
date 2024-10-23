package validator

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/beef"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
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
	ValidateTransaction(ctx context.Context, tx *sdkTx.Transaction, feeValidation FeeValidation, scriptValidation ScriptValidation) error
}

type BeefValidator interface {
	ValidateTransaction(ctx context.Context, b *beef.BEEF, feeValidation FeeValidation, scriptValidation ScriptValidation) (*sdkTx.Transaction, error)
}
