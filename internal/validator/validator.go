package validator

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/libsv/go-bt/v2"
)

type FeeValidation byte

const (
	NoneFeeValidation FeeValidation = iota
	StandardFeeValidation
	CumulativeFeeValidation
)

type ScriptValidation byte

const (
	NoneScriptValidation ScriptValidation = iota
	StandardScriptValidation
)

type DefaultValidator interface {
	ValidateTransaction(ctx context.Context, tx *bt.Tx, feeValidation FeeValidation, scriptValidation ScriptValidation) error
}

type BeefValidator interface {
	ValidateTransaction(ctx context.Context, beef *beef.BEEF, feeValidation FeeValidation, scriptValidation ScriptValidation) (*bt.Tx, error)
}
